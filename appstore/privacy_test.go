package appstore

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrivacyNutritionLabels(t *testing.T) {
	// First we need to get the token
	token, err := GetToken(context.Background(), http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}

	const ClockId = AppId(1584215688)

	labels, err := ScrapePrivacy(context.Background(), http.DefaultClient, token, []AppId{ClockId})
	if err != nil {
		t.Fatal(err)
	}

	clockNutritionLabels := labels[ClockId]

	// The clock should have one "Data Not Linked to You" label
	assert.Len(t, clockNutritionLabels, 1)

	clockLabel := clockNutritionLabels[0]
	assert.Equal(t, "DATA_NOT_LINKED_TO_YOU", clockLabel.Identifier)

	// The clock has one "privacy purpose": analytics
	expected := PrivacyPurpose{
		Identifier: "ANALYTICS",
		DataCategories: []PrivacyDataCategories{
			{Identifier: "IDENTIFIERS", DataTypes: []string{"Device ID"}},
			{Identifier: "USAGE_DATA", DataTypes: []string{"Product Interaction"}},
		},
	}

	assert.Equal(t, expected, clockLabel.Purposes[0])
}
