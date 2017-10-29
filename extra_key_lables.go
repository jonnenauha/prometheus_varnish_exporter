package main

type extraLabelValues map[string]string

func (exlv extraLabelValues) getVal(label string) (string, bool) {
	val, ok := exlv[label]
	if ok {
		return val, ok
	} else {
		return "", false
	}
}

func (exlv extraLabelValues) getLabelValues() map[string]string {
	return map[string]string(exlv)
}

func (exlv extraLabelValues) add(key string, val string) bool {
	oldval, ok := exlv[key]
	if ok {
		if oldval == val {
			return false
		}
	}
	exlv[key] = val
	return true
}

func newExtraLabelValues() extraLabelValues {
	ex := make(map[string]string)
	return extraLabelValues(ex)
}
