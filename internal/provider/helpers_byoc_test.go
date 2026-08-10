package provider

import "testing"

func TestDeterministicIdempotencyKeyIsStableAndIdentityBound(t *testing.T) {
	first := deterministicIdempotencyKey("tf-byoc-node-create", "connection", "aws", "eu-west-1", "growth", "app")
	second := deterministicIdempotencyKey("tf-byoc-node-create", "connection", "aws", "eu-west-1", "growth", "app")
	changed := deterministicIdempotencyKey("tf-byoc-node-create", "connection", "aws", "eu-west-1", "growth", "worker")
	if first != second {
		t.Fatalf("same resource identity produced different idempotency keys: %q != %q", first, second)
	}
	if first == changed {
		t.Fatal("different resource identities produced the same idempotency key")
	}
	if len(first) != len("tf-byoc-node-create-")+64 {
		t.Fatalf("unexpected key length: %d", len(first))
	}
}
