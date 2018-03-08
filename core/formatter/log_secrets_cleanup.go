package formatter

import "github.com/sirupsen/logrus"

type SecretsCleanupHook struct{}

func (s *SecretsCleanupHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

func (s *SecretsCleanupHook) Fire(entry *logrus.Entry) error {
	entry.Message = ScrubSecrets(entry.Message)
	return nil
}

func AddSecretsCleanupLogHook() {
	logrus.AddHook(&SecretsCleanupHook{})
}
