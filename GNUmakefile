default: testacc

# Run acceptance tests
.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

build:
	mkdir -p ~/.terraform.d/plugins/terraform.local/local/cockroachgke/1.0.0/darwin_arm64
	go build -o terraform-provider-cockroachgke
	chmod +x terraform-provider-cockroachgke
	mv terraform-provider-cockroachgke ~/.terraform.d/plugins/terraform.local/local/cockroachgke/1.0.0/darwin_arm64/terraform-provider-cockroachgke_v1.0.0