package kubernetes

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jpillora/backoff"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const maxLogLineBufferSize = 16 * 1024

type logStreamProvider interface {
	LogStream(since *time.Time) (io.ReadCloser, error)
	String() string
}

type kubernetesLogStreamProvider struct {
	client    *kubernetes.Clientset
	namespace string
	pod       string
	container string
}

func (s *kubernetesLogStreamProvider) LogStream(since *time.Time) (io.ReadCloser, error) {
	var sinceTime metav1.Time
	if since != nil {
		sinceTime = metav1.NewTime(*since)
	}

	return s.client.
		CoreV1().
		Pods(s.namespace).
		GetLogs(s.pod, &api.PodLogOptions{
			Container:  s.container,
			SinceTime:  &sinceTime,
			Follow:     true,
			Timestamps: true,
		}).Stream()
}

func (s *kubernetesLogStreamProvider) String() string {
	return fmt.Sprintf("%s/%s/%s", s.namespace, s.pod, s.container)
}

type logProcessor interface {
	// Listen listens for log lines
	// consumers should read from the channel until it's closed
	// otherwise, risk leaking goroutines
	Listen(ctx context.Context) <-chan string
}

type timestampsSet map[int64]struct{}

// kubernetesLogProcessor processes log from multiple containers in a pod and sends them out through one channel.
// It also tries to reattach to the log constantly, stopping only when the passed context is cancelled.
type kubernetesLogProcessor struct {
	backoff      backoff.Backoff
	logger       *common.BuildLogger
	logProviders []logStreamProvider
}

type kubernetesLogProcessorPodConfig struct {
	namespace  string
	pod        string
	containers []string
}

func newKubernetesLogProcessor(
	client *kubernetes.Clientset,
	backoff backoff.Backoff,
	logger *common.BuildLogger,
	podCfg kubernetesLogProcessorPodConfig,
) logProcessor {
	logProviders := make([]logStreamProvider, len(podCfg.containers))
	for i, container := range podCfg.containers {
		logProviders[i] = &kubernetesLogStreamProvider{
			client:    client,
			namespace: podCfg.namespace,
			pod:       podCfg.pod,
			container: container,
		}
	}

	return &kubernetesLogProcessor{
		backoff:      backoff,
		logger:       logger,
		logProviders: logProviders,
	}
}

func (l *kubernetesLogProcessor) Listen(ctx context.Context) <-chan string {
	outCh := make(chan string)

	var wg sync.WaitGroup
	for _, logProvider := range l.logProviders {
		wg.Add(1)
		go func(logProvider logStreamProvider) {
			defer wg.Done()
			l.attach(ctx, logProvider, outCh)
		}(logProvider)
	}

	go func() {
		wg.Wait()
		close(outCh)
	}()

	return outCh
}

func (l *kubernetesLogProcessor) attach(ctx context.Context, logProvider logStreamProvider, outputCh chan string) {
	var sinceTime time.Time
	var attempt int32

	processedTimestamps := timestampsSet{}

	for {
		select {
		// If we have to exit, check for that before trying to (re)attach
		case <-ctx.Done():
			return
		default:
		}

		if attempt > 0 {
			backoffDuration := l.backoff.ForAttempt(float64(attempt))
			l.logger.Debugln(fmt.Sprintf("Backing off reattaching log for %s for %s", logProvider, backoffDuration))
			time.Sleep(backoffDuration)
		}

		attempt++

		logs, err := logProvider.LogStream(&sinceTime)
		if err != nil {
			l.logger.Warningln(fmt.Sprintf("Error attaching to log %s: %s. Retrying...", logProvider, err))
			continue
		}

		// If we succeed in connecting to the stream, set the attempts to 1, so that next time we try to reconnect
		// as soon as possible but also still have some delay, so we don't bombard kubernetes with requests in case
		// readLogs fails too frequently
		attempt = 1

		sinceTime, err = l.readLogs(ctx, logs, processedTimestamps, sinceTime, outputCh)
		if err != nil {
			l.logger.Warningln(fmt.Sprintf("Error reading log for %s: %s. Retrying...", logProvider, err))
		}

		err = logs.Close()
		if err != nil {
			l.logger.Warningln(fmt.Sprintf("Error when closing Kubernetes log stream for %s. %v", logProvider, err))
		}
	}
}

func (l *kubernetesLogProcessor) readLogs(
	ctx context.Context, logs io.Reader, timestamps timestampsSet,
	sinceTime time.Time, outputCh chan string,
) (time.Time, error) {
	logsScanner, linesCh := l.scan(ctx, logs)

	for {
		select {
		case <-ctx.Done():
			return sinceTime, nil
		case line, more := <-linesCh:
			if !more {
				return sinceTime, logsScanner.Err()
			}

			newSinceTime, logLine, parseErr := l.parseLogLine(line)
			if parseErr != nil {
				return sinceTime, parseErr
			}

			// Cache log lines based on their timestamp. Since the reattaching precision of kubernetes logs is seconds
			// we need to make sure that we won't process a line twice in case we reattach and get it again
			// The size of the int64 key is 8 bytes and the empty struct is 0. Even with a million logs we should be fine
			// using only 8 MB of memory.
			// Since there's a network delay before a log line is processed by kubernetes itself,
			// it's impossible to get two log lines with the same timestamp
			timeUnix := newSinceTime.UnixNano()
			_, alreadyProcessed := timestamps[timeUnix]
			if alreadyProcessed {
				continue
			}
			timestamps[timeUnix] = struct{}{}

			sinceTime = newSinceTime
			outputCh <- logLine
		}
	}
}

// splitLinesStartingWithDateWithMaxBufferSize splits docker logs at a specified buffer size.
// As seen here https://github.com/moby/moby/issues/32923 the default log line limit for the docker daemon is
// 16k. This limit is at the time of writing this unconfigurable and appears to continue being that way in the future.
// This log line limit splits log lines at the 16k byte mark, so if we have a log line longer than 16k it will be split
// into two lines. For now, this method relies that we know this limit to be 16k. If docker ever makes this configurable
// we could make it as well.
// In addition to the lines splitting there's another issue when using --timestamps with docker/kubectl logs.
// It's explained here https://github.com/kubernetes/kubernetes/issues/77603. As an end result we have the following logs
// (LOG-TIMESTAMP) THE-FIRST-16k-OF-THE-LOG-LINE(LOG-TIMESTAMP) THE-REST-OF-THE-LOG-LINE
// Additionally the (LOG-TIMESTAMP) is the same for both lines. Usually docker gives us different timestamps for each line
// but here we get the timestamp of the start of the line.
// This function takes care to concatenate split log lines into a single line that correctly starts with a single timestamp
// which is the expected behavior from docker/kubectl logs. The only limitation is that the line can't be larger than maxLineBufferSize.
// If https://github.com/kubernetes/kubernetes/issues/77603 gets fixed this function will continue working correctly.
// IMPORTANT: maxBufferSize must be smaller than the Scanner's buffer size, which by default is bufio.MaxScanTokenSize.
// otherwise bufio.ErrTooLong will be hit.
func splitLinesStartingWithDateWithMaxBufferSize(maxBufferSize int, maxLineBufferSize int) bufio.SplitFunc {
	var lineBuf bytes.Buffer
	return func(data []byte, atEOF bool) (int, []byte, error) {
		// Get the end of the timestamp the log line is in the format
		// 2020-04-01T00:39:20.505277986Z log_line
		// if the data doesn't start with a timestamp the offset will be 0.
		// In that case, this means this is a continuation of the previous line.
		offset := bufferDateOffset(data)
		maxBufferSizeWithDateOffset := maxBufferSize + offset
		if maxBufferSizeWithDateOffset > len(data) {
			maxBufferSizeWithDateOffset = len(data)
		}

		var advance int
		var token []byte
		var err error
		// This is the general case. Most log lines will be smaller than 16k
		// in that case we offload the scanning of the whole data to the default bufio.ScanLines
		if len(data) <= maxBufferSize {
			advance, token, err = bufio.ScanLines(data, atEOF)
			// If we get no token back this means a new line wasn't found
			// request more data from the Scanner
			if (advance == 0 && len(token) == 0) || err != nil {
				return 0, nil, err
			}
		} else {
			// If the size of the log is larger than the limit we try to find a new line only
			// within the allowed limits.
			// The +1 with the offset is for a possible newline character, since it's not included in the
			// 16k limit.
			advance, token, err = bufio.ScanLines(data[:maxBufferSizeWithDateOffset+1], atEOF)
			if err != nil {
				return 0, nil, err
			}

			// If we didn't find a newline character we add the first 16k of the data buffer to the line buffer.
			if advance == 0 && len(token) == 0 {
				// If the buffered line is empty, add the timestamp to the start of it. We rely on timestamps to dedupe lines
				// when reattaching so it's important.
				if lineBuf.Len() == 0 {
					lineBuf.Write(data[:offset])
				}

				// Get the whole 16k part of the line from the buffer. The timestamp isn't included in these 16k since
				// it's added as an afterthought by docker.
				linePart := data[offset:maxBufferSizeWithDateOffset]
				if lineBuf.Len()+len(linePart) > maxLineBufferSize {
					return 0, nil, errors.New("exceeded log line limit")
				}
				lineBuf.Write(linePart)

				// This allows us to tell the Scanner to advance the buffer without returning results.
				// This way we request more bytes while discarding the old ones.
				return maxBufferSizeWithDateOffset, nil, nil
			}
		}

		// If we found a new line in the data buffer check if we have already buffered a part of the line
		// if we did, we add this last part to the buffered part.
		if lineBuf.Len() > 0 {
			// Remove the timestamp from each buffer. We only care abut the first timestamp since all the others are the same
			// and don't bring value to us, only break up the log.
			// TODO: add check to only remove the timestamp if it's the same as the beginning of the buffered line
			// this should avoid any potential edge cases where there's a timestamp on a place in the log which happens to be
			// at the start of the buffer while it's being split into parts.
			tokenWithoutDate := token[offset:]
			if lineBuf.Len()+len(tokenWithoutDate) > maxLineBufferSize {
				return 0, nil, errors.New("exceeded log line limit")
			}

			line := make([]byte, lineBuf.Len())
			copy(line, lineBuf.Bytes())
			lineBuf.Reset()
			token = append(line, tokenWithoutDate...)
		}

		return advance, token, err
	}
}

func bufferDateOffset(buf []byte) int {
	firstChar, _ := utf8.DecodeRune(buf)
	if !unicode.IsDigit(firstChar) {
		return 0
	}

	dateEndIndex := bytes.Index(buf, []byte(" "))
	if dateEndIndex == -1 {
		return 0
	}

	_, err := time.Parse(time.RFC3339Nano, string(buf[:dateEndIndex]))
	if err != nil {
		return 0
	}

	return dateEndIndex + 1
}

func (l *kubernetesLogProcessor) scan(ctx context.Context, logs io.Reader) (*bufio.Scanner, <-chan string) {
	logsScanner := bufio.NewScanner(logs)
	logsScanner.Split(splitLinesStartingWithDateWithMaxBufferSize(maxLogLineBufferSize, common.DefaultTraceOutputLimit))

	linesCh := make(chan string)
	go func() {
		defer close(linesCh)

		// This goroutine will exit when the calling method closes the logs stream or the context is cancelled
		for logsScanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case linesCh <- logsScanner.Text():
			}
		}
	}()

	return logsScanner, linesCh
}

// Each line starts with an RFC3339Nano formatted date. We need this date to resume the log from that point
// if we detach for some reason. The format is "2020-01-30T16:28:25.479904159Z log line continues as normal"
// also the line doesn't include the "\n" at the end.
func (l *kubernetesLogProcessor) parseLogLine(line string) (time.Time, string, error) {
	if len(line) == 0 {
		return time.Time{}, "", fmt.Errorf("empty line: %w", io.EOF)
	}

	// Get the index where the date ends and parse it
	dateEndIndex := strings.Index(line, " ")

	// This should not happen but in case there's no space in the log try to parse them all as a date
	// this way we could at least get an error without going out of the bounds of the line
	var date string
	if dateEndIndex > -1 {
		date = line[:dateEndIndex]
	} else {
		date = line
	}

	parsedDate, err := time.Parse(time.RFC3339Nano, date)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid log timestamp: %w", err)
	}

	// We are sure this will never get out of bounds since we know that kubernetes always inserts a
	// date and space directly after. So if we get an empty log line, this slice will be simply empty
	logLine := line[dateEndIndex+1:]

	return parsedDate, logLine, nil
}
