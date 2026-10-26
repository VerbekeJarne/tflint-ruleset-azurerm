package resourceproviders

import "testing"

func TestSet(t *testing.T) {
	s := NewSet("Microsoft.Dashboard")
	s.Add("microsoft.storage")

	for _, namespace := range []string{"Microsoft.Dashboard", "microsoft.dashboard", "MICROSOFT.STORAGE"} {
		if !s.Contains(namespace) {
			t.Errorf("expected set to contain %q", namespace)
		}
	}
	if s.Contains("Microsoft.Dashboardd") {
		t.Errorf("expected set not to contain %q", "Microsoft.Dashboardd")
	}
}

func TestRegistrationSet(t *testing.T) {
	for _, mode := range []string{"none", "core", "extended", "all", "legacy"} {
		if _, ok := RegistrationSet(mode); !ok {
			t.Errorf("expected registration set for mode %q", mode)
		}
	}
	if _, ok := RegistrationSet("invalid"); ok {
		t.Error("expected no registration set for mode \"invalid\"")
	}
}

func TestRegistrationSetReturnsCopy(t *testing.T) {
	s, _ := RegistrationSet("core")
	s.Add("Microsoft.Dashboard")

	if fresh, _ := RegistrationSet("core"); fresh.Contains("Microsoft.Dashboard") {
		t.Error("expected modifying a registration set not to affect other callers")
	}
}

func TestResources(t *testing.T) {
	if len(Resources) == 0 {
		t.Fatal("expected generated resources")
	}
	for resourceType, namespaces := range Resources {
		if len(namespaces) == 0 {
			t.Errorf("expected namespaces for %s", resourceType)
		}
	}
}
