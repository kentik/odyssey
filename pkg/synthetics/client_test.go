package synthetics

import (
	"context"
	"os"
	"testing"
)

func TestSyntheticsClientAgents(t *testing.T) {
	var (
		email string
		token string
	)

	client := NewClient(os.Getenv("KENTIK_EMAIL"), os.Getenv("KENTIK_API_TOKEN"))
	ctx := context.Background()

	if _, err := client.Agents(ctx); err != nil {
		t.Fatal(err)
	}
}
