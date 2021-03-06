package v1alpha1

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetRequirementStatus(t *testing.T) {
	csv := ClusterServiceVersion{}
	status := []RequirementStatus{{Group: "test", Version: "test", Kind: "Test", Name: "test", Status: "test", UUID: "test"}}
	csv.SetRequirementStatus(status)
	require.Equal(t, csv.Status.RequirementStatus, status)
}

func TestSetPhase(t *testing.T) {
	tests := []struct {
		currentPhase      ClusterServiceVersionPhase
		currentConditions []ClusterServiceVersionCondition
		inPhase           ClusterServiceVersionPhase
		outPhase          ClusterServiceVersionPhase
		description       string
	}{
		{
			currentPhase:      "",
			currentConditions: []ClusterServiceVersionCondition{},
			inPhase:           CSVPhasePending,
			outPhase:          CSVPhasePending,
			description:       "NoPhase",
		},
		{
			currentPhase:      CSVPhasePending,
			currentConditions: []ClusterServiceVersionCondition{{Phase: CSVPhasePending}},
			inPhase:           CSVPhasePending,
			outPhase:          CSVPhasePending,
			description:       "SamePhase",
		},
		{
			currentPhase:      CSVPhasePending,
			currentConditions: []ClusterServiceVersionCondition{{Phase: CSVPhasePending}},
			inPhase:           CSVPhaseInstalling,
			outPhase:          CSVPhaseInstalling,
			description:       "DifferentPhase",
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			csv := ClusterServiceVersion{
				Status: ClusterServiceVersionStatus{
					Phase:      tt.currentPhase,
					Conditions: tt.currentConditions,
				},
			}
			csv.SetPhase(tt.inPhase, "test", "test", metav1.Now())
			require.EqualValues(t, tt.outPhase, csv.Status.Phase)
		})
	}
}

func TestIsObsolete(t *testing.T) {
	tests := []struct {
		currentPhase      ClusterServiceVersionPhase
		currentConditions []ClusterServiceVersionCondition
		out               bool
		description       string
	}{
		{
			currentPhase:      "",
			currentConditions: []ClusterServiceVersionCondition{},
			out:               false,
			description:       "NoPhase",
		},
		{
			currentPhase:      CSVPhasePending,
			currentConditions: []ClusterServiceVersionCondition{{Phase: CSVPhasePending}},
			out:               false,
			description:       "Pending",
		},
		{
			currentPhase:      CSVPhaseReplacing,
			currentConditions: []ClusterServiceVersionCondition{{Phase: CSVPhaseReplacing, Reason: CSVReasonBeingReplaced}},
			out:               true,
			description:       "Replacing",
		},
		{
			currentPhase:      CSVPhaseDeleting,
			currentConditions: []ClusterServiceVersionCondition{{Phase: CSVPhaseDeleting, Reason: CSVReasonReplaced}},
			out:               true,
			description:       "CSVPhaseDeleting",
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			csv := ClusterServiceVersion{
				Status: ClusterServiceVersionStatus{
					Phase:      tt.currentPhase,
					Conditions: tt.currentConditions,
				},
			}
			require.Equal(t, csv.IsObsolete(), tt.out)
		})
	}
}

func TestSupports(t *testing.T) {
	tests := []struct {
		description       string
		installModeSet    InstallModeSet
		operatorNamespace string
		namespaces        []string
		expectedErr       error
	}{
		{
			description: "NoNamespaces",
			installModeSet: InstallModeSet{
				InstallModeTypeOwnNamespace:    true,
				InstallModeTypeSingleNamespace: true,
				InstallModeTypeMultiNamespace:  true,
				InstallModeTypeAllNamespaces:   true,
			},
			operatorNamespace: "operators",
			namespaces:        []string{},
			expectedErr:       nil,
		},
		{
			description: "OneNamespace",
			installModeSet: InstallModeSet{
				InstallModeTypeOwnNamespace:    true,
				InstallModeTypeSingleNamespace: true,
				InstallModeTypeMultiNamespace:  true,
				InstallModeTypeAllNamespaces:   true,
			},
			operatorNamespace: "operators",
			namespaces:        []string{"ns-0"},
			expectedErr:       nil,
		},
		{
			description: "MultipleNamespaces/MultiNamespaceUnsupported",
			installModeSet: InstallModeSet{
				InstallModeTypeOwnNamespace:    true,
				InstallModeTypeSingleNamespace: true,
				InstallModeTypeMultiNamespace:  false,
				InstallModeTypeAllNamespaces:   true,
			},
			operatorNamespace: "operators",
			namespaces:        []string{"ns-0", "ns-1"},
			expectedErr:       fmt.Errorf("%s InstallModeType not supported, cannot configure to watch 2 namespaces", InstallModeTypeMultiNamespace),
		},
		{
			description: "MultipleNamespaces/OwnNamespaceUnsupported",
			installModeSet: InstallModeSet{
				InstallModeTypeOwnNamespace:    false,
				InstallModeTypeSingleNamespace: true,
				InstallModeTypeMultiNamespace:  true,
				InstallModeTypeAllNamespaces:   true,
			},
			operatorNamespace: "operators",
			namespaces:        []string{"ns-0", "ns-1", "operators"},
			expectedErr:       fmt.Errorf("%s InstallModeType not supported, cannot configure to watch own namespace", InstallModeTypeOwnNamespace),
		},
		{
			description: "SingleNamespace/SingleAndMultiNamespaceUnsupported",
			installModeSet: InstallModeSet{
				InstallModeTypeOwnNamespace:    true,
				InstallModeTypeSingleNamespace: false,
				InstallModeTypeMultiNamespace:  false,
				InstallModeTypeAllNamespaces:   true,
			},
			operatorNamespace: "operators",
			namespaces:        []string{"ns-0"},
			expectedErr:       fmt.Errorf("%s InstallModeType not supported, cannot configure to watch one namespace", InstallModeTypeSingleNamespace),
		},
		{
			description: "SingleNamespace/MultiNamespaceDecomposes",
			installModeSet: InstallModeSet{
				InstallModeTypeOwnNamespace:    true,
				InstallModeTypeSingleNamespace: false,
				InstallModeTypeMultiNamespace:  true,
				InstallModeTypeAllNamespaces:   true,
			},
			operatorNamespace: "operators",
			namespaces:        []string{"ns-0"},
			expectedErr:       nil,
		},
		{
			description: "SingleNamespace/OwnNamespaceUnsupported",
			installModeSet: InstallModeSet{
				InstallModeTypeOwnNamespace:    false,
				InstallModeTypeSingleNamespace: true,
				InstallModeTypeMultiNamespace:  true,
				InstallModeTypeAllNamespaces:   true,
			},
			operatorNamespace: "operators",
			namespaces:        []string{"operators"},
			expectedErr:       fmt.Errorf("%s InstallModeType not supported, cannot configure to watch own namespace", InstallModeTypeOwnNamespace),
		},
		{
			description: "AllNamespaces/AllNamespacesSupported",
			installModeSet: InstallModeSet{
				InstallModeTypeOwnNamespace:    true,
				InstallModeTypeSingleNamespace: true,
				InstallModeTypeMultiNamespace:  true,
				InstallModeTypeAllNamespaces:   true,
			},
			operatorNamespace: "operators",
			namespaces:        []string{corev1.NamespaceAll},
			expectedErr:       nil,
		},
		{
			description: "AllNamespaces/AllNamespacesUnsupported",
			installModeSet: InstallModeSet{
				InstallModeTypeOwnNamespace:    true,
				InstallModeTypeSingleNamespace: true,
				InstallModeTypeMultiNamespace:  true,
				InstallModeTypeAllNamespaces:   false,
			},
			operatorNamespace: "operators",
			namespaces:        []string{corev1.NamespaceAll},
			expectedErr:       fmt.Errorf("%s InstallModeType not supported, cannot configure to watch all namespaces", InstallModeTypeAllNamespaces),
		},
		{
			description:       "NoNamespaces/EmptyInstallModeSet",
			installModeSet:    InstallModeSet{},
			operatorNamespace: "",
			namespaces:        []string{},
			expectedErr:       nil,
		},
		{
			description:       "MultipleNamespaces/EmptyInstallModeSet",
			installModeSet:    InstallModeSet{},
			operatorNamespace: "operators",
			namespaces:        []string{"ns-0", "ns-1"},
			expectedErr:       fmt.Errorf("%s InstallModeType not supported, cannot configure to watch 2 namespaces", InstallModeTypeMultiNamespace),
		},
		{
			description:       "SingleNamespace/EmptyInstallModeSet",
			installModeSet:    InstallModeSet{},
			operatorNamespace: "operators",
			namespaces:        []string{"ns-0"},
			expectedErr:       fmt.Errorf("%s InstallModeType not supported, cannot configure to watch one namespace", InstallModeTypeSingleNamespace),
		},
		{
			description:       "AllNamespaces/EmptyInstallModeSet",
			installModeSet:    InstallModeSet{},
			operatorNamespace: "operators",
			namespaces:        []string{corev1.NamespaceAll},
			expectedErr:       fmt.Errorf("%s InstallModeType not supported, cannot configure to watch all namespaces", InstallModeTypeAllNamespaces),
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			err := tt.installModeSet.Supports(tt.operatorNamespace, tt.namespaces)
			require.Equal(t, tt.expectedErr, err)
		})
	}
}
