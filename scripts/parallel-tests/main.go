package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

type Buckets []Bucket

func (bs *Buckets) Add(tc Item) {
	chosen := 0
	lowest := math.MaxFloat64
	for idx, b := range *bs {
		score := b.Score(tc)
		if score < lowest {
			lowest = score
			chosen = idx
		}
	}

	(*bs)[chosen].Items = append((*bs)[chosen].Items, tc)
}

type Bucket struct {
	Items []Item
}

func (b *Bucket) TotalTime() float64 {
	var total float64
	for _, item := range b.Items {
		total += item.Timing
	}

	return total
}

func (b *Bucket) Groups() map[string][]string {
	group := make(map[string][]string)
	for _, item := range b.Items {
		group[item.Pkg] = append(group[item.Pkg], item.Name)
	}

	return group
}

func (b *Bucket) Score(tc Item) float64 {
	// score starts with how many items we currently have
	score := float64(len(b.Items))

	for _, item := range b.Items {
		// influenced by how much time would be added
		score += item.Timing + tc.Timing

		// influenced by how many similar items we already have
		if item.Pkg == tc.Pkg {
			score -= 1
		}
	}

	return score
}

type Item struct {
	Pkg    string
	Name   string
	Timing float64
}

type TestReport struct {
	Timings []TestTiming
}

type Items []Item

func (t Items) Len() int {
	return len(t)
}

func (t Items) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t Items) Less(i, j int) bool {
	return t[i].Timing < t[j].Timing
}

func (tr *TestReport) GetTiming(pkg, name string) float64 {
	var max float64
	for _, tc := range tr.Timings {
		if strings.HasSuffix(pkg, tc.Class) && tc.Name == name {
			if tc.Timing > max {
				max = tc.Timing
			}
		}
	}

	return max
}

type TestTiming struct {
	Class  string  `json:"classname"`
	Name   string  `json:"name"`
	Timing float64 `json:"execution_time"`
}

func main() {
	// TEST_DEFINITIONS_NAME
	// TEST_DEFINITIONS_TAGS
	// TEST_DEFINITIONS_PARALLEL
	fmt.Println("Fetching previous test report...")
	report, err := getTestReport("250833")
	if err != nil {
		fmt.Printf("error fetching previous test timings: %v", err)
	}

	args := []string{"test"}
	if os.Getenv("TEST_DEFINITIONS_TAGS") != "" {
		args = append(args, "-tags", os.Getenv("TEST_DEFINITIONS_TAGS"))
	}
	args = append(args, "-list", "Test", "-json", "./...")

	fmt.Println("Fetching go tests...")
	cmd := exec.Command("go", args...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	count, _ := strconv.Atoi(os.Getenv("TEST_DEFINITIONS_PARALLEL"))
	if count == 0 {
		count = 1
	}

	fmt.Printf("Creating %d test buckets\n", count)
	buckets := make(Buckets, count)

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	var result struct {
		Action  string
		Package string
		Output  string
	}

	var items Items
	dec := json.NewDecoder(out)
	for {
		err := dec.Decode(&result)
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		if result.Action != "output" {
			continue
		}

		if !strings.HasPrefix(result.Output, "Test") {
			continue
		}

		name := strings.TrimSpace(result.Output)
		items = append(items, Item{result.Package, name, report.GetTiming(result.Package, name)})
	}

	sort.Sort(sort.Reverse(items))
	for _, item := range items {
		buckets.Add(item)
	}

	if err := cmd.Wait(); err != nil {
		panic(err)
	}

	testDefinitionsName := os.Getenv("TEST_DEFINITIONS_NAME")
	if testDefinitionsName == "" {
		testDefinitionsName = "testsdefinitions"
	}

	fmt.Println("Splitting...")
	for idx, b := range buckets {
		fmt.Printf("bucket(%d), %.2f seconds, %d tests\n", idx, b.TotalTime(), len(b.Items))

		f, err := os.Create(fmt.Sprintf("%s-%d", testDefinitionsName, idx+1))
		if err != nil {
			panic(err)
		}

		for pkg, item := range b.Groups() {
			fmt.Fprintln(f, pkg, strings.Join(item, "|"))
		}

		/*for pkg, item := range b.Groups() {
			hash := sha256.Sum256([]byte(pkg))
			fmt.Fprintln(
				f,
				"gotestsum",
				"--junitfile",
				fmt.Sprintf(".testoutput/%x.%d.output.xml", hash, idx),
				"--",
				testFlags,
				"-run",
				`"`+strings.Join(item, "|")+`"`,
				"-coverprofile",
				fmt.Sprintf(".cover/%x.%d.cover.profile", hash, idx),
				"-coverpkg",
				mainPkg+"/...",
				"-timeout 30m",
				pkg,
			)
		}*/

		f.Close()
	}
}

func getTestReport(projectID string) (*TestReport, error) {
	report := &TestReport{}

	pID, err := getSuccessfulLatestPipelineID(projectID)
	if err != nil {
		return report, fmt.Errorf("fetching latest pipeline: %w", err)
	}

	var results struct {
		Suites []struct {
			Cases []TestTiming `json:"test_cases"`
		} `json:"test_suites"`
	}

	err = get(fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/pipelines/%d/test_report", projectID, pID), &results)
	if err != nil {
		return report, err
	}

	for _, suite := range results.Suites {
		report.Timings = append(report.Timings, suite.Cases...)
	}

	return report, nil
}

func getSuccessfulLatestPipelineID(projectID string) (uint64, error) {
	var results []struct {
		ID uint64 `json:"id"`
	}

	err := get(fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/pipelines?ref=master&status=success", projectID), &results)
	if err != nil {
		return 0, err
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("no results found")
	}

	return results[0].ID, nil
}

func get(url string, results interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 200))

		return fmt.Errorf("non-200 response: %s", string(snippet))
	}

	if err := json.NewDecoder(resp.Body).Decode(results); err != nil {
		return fmt.Errorf("decoding results: %w", err)
	}

	return nil
}
