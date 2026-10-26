// Package resourceproviders contains the Azure resource provider namespaces used by azurerm resources,
// and the namespaces registered by each resource_provider_registrations mode of the azurerm provider.
package resourceproviders

import "strings"

//go:generate go run -tags generators ./generator

// RegisteredByDefault contains the namespaces Azure registers in every subscription.
// https://learn.microsoft.com/azure/azure-resource-manager/management/azure-services-resource-providers
var RegisteredByDefault = []string{
	"Microsoft.ADHybridHealthService",
	"Microsoft.Authorization",
	"Microsoft.Billing",
	"Microsoft.ChangeSafety",
	"Microsoft.ClassicSubscription",
	"Microsoft.Commerce",
	"Microsoft.Consumption",
	"Microsoft.CostManagement",
	"Microsoft.Features",
	"Microsoft.MarketplaceOrdering",
	"Microsoft.Portal",
	"Microsoft.ResourceGraph",
	"Microsoft.ResourceNotification",
	"Microsoft.Resources",
	"Microsoft.SerialConsole",
	"microsoft.support",
}

// Set is a case-insensitive set of resource provider namespaces.
type Set map[string]struct{}

// NewSet returns a set containing the given namespaces.
func NewSet(namespaces ...string) Set {
	s := Set{}
	s.Add(namespaces...)
	return s
}

// Add adds the given namespaces to the set.
func (s Set) Add(namespaces ...string) {
	for _, namespace := range namespaces {
		s[strings.ToLower(namespace)] = struct{}{}
	}
}

// Contains reports whether the set contains the given namespace, ignoring case.
func (s Set) Contains(namespace string) bool {
	_, ok := s[strings.ToLower(namespace)]
	return ok
}

// RegistrationSet returns the namespaces registered by the given resource_provider_registrations mode.
func RegistrationSet(mode string) (Set, bool) {
	namespaces, ok := Registrations[mode]
	if !ok {
		return nil, false
	}
	return NewSet(namespaces...), true
}
