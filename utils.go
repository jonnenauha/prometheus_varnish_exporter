package main

import (
	"fmt"
	"os"
	"strings"
)

// logging

func logRaw(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func logTitle(format string, args ...interface{}) {
	logInfo(format, args...)

	title := strings.Repeat("-", len(fmt.Sprintf(format, args...)))
	if len(title) > 0 {
		logInfo(title)
	}
}

func logInfo(format string, args ...interface{}) {
	if StartParams.Raw {
		logRaw(format, args...)
	} else {
		logger.Printf(format, args...)
	}
}

func logWarn(format string, args ...interface{}) {
	format = "[WARN] " + format
	if StartParams.Raw {
		logRaw(format, args...)
	} else {
		logger.Printf(format, args...)
	}
}

func logError(format string, args ...interface{}) {
	format = "[ERROR] " + format
	if StartParams.Raw {
		logRaw(format, args...)
	} else {
		logger.Printf(format, args...)
	}
}

func logFatal(format string, args ...interface{}) {
	format = "[FATAL] " + format
	if StartParams.Raw {
		logRaw(format, args...)
	} else {
		logger.Printf(format, args...)
	}
	os.Exit(1)
}

func logFatalError(err error) {
	if err != nil {
		logFatal(err.Error())
	}
}

// strings

type caseSensitivity int

const (
	caseSensitive   caseSensitivity = 0
	caseInsensitive caseSensitivity = 1
)

func startsWith(str, prefix string, cs caseSensitivity) bool {
	if cs == caseSensitive {
		return strings.HasPrefix(str, prefix)
	}
	return strings.HasPrefix(strings.ToLower(str), strings.ToLower(prefix))
}

func startsWithAny(str string, prefixes []string, cs caseSensitivity) bool {
	for _, prefix := range prefixes {
		if startsWith(str, prefix, cs) {
			return true
		}
	}
	return false
}

func endsWith(str, postfix string, cs caseSensitivity) bool {
	if cs == caseSensitive {
		return strings.HasSuffix(str, postfix)
	}
	return strings.HasSuffix(strings.ToLower(str), strings.ToLower(postfix))
}

func endsWithAny(str string, postfixes []string, cs caseSensitivity) bool {
	for _, postfix := range postfixes {
		if endsWith(str, postfix, cs) {
			return true
		}
	}
	return false
}
