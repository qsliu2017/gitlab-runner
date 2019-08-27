package process

func testKillerTestCases() map[string]testKillerTestCase {
	return map[string]testKillerTestCase{
		"command terminated": {
			alreadyStopped: false,
			skipTerminate:  true,
			expectedError:  "exit status 1",
		},
		"command not terminated": {
			alreadyStopped: false,
			skipTerminate:  false,
			expectedError:  "exit status 1",
		},
		"command already stopped": {
			alreadyStopped: true,
			expectedError:  "exit status 1",
		},
	}
}
