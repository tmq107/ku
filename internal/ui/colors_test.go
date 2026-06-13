package ui

import "testing"

func TestStatusClass(t *testing.T) {
	cases := map[string]int{
		"Running":           classGood,
		"Completed":         classGood,
		"Ready":             classGood,
		"Bound":             classGood,
		"Active":            classGood,
		"True":              classGood,
		"Succeeded":         classGood,
		"Pending":           classWarn,
		"ContainerCreating": classWarn,
		"Terminating":       classWarn,
		"Init:0/2":          classWarn,
		"NotReady":          classWarn,
		"Unknown":           classWarn,
		"CrashLoopBackOff":  classBad,
		"Error":             classBad,
		"ImagePullBackOff":  classBad,
		"OOMKilled":         classBad,
		"Evicted":           classBad,
		"Failed":            classBad,
		"Unschedulable":     classBad,
		"":                  classNone,
		"Banana":            classNone,
	}
	for v, want := range cases {
		if got := statusClass(v); got != want {
			t.Errorf("statusClass(%q) = %d, want %d", v, got, want)
		}
	}
}
