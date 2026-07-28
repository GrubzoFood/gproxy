REGION ?= ap-south-2
BUCKET ?= gproxy-tfstate

.PHONY: bootstrap-backend destroy-backend \
        init plan apply apply-auto destroy destroy-auto \
        fmt validate clean init-plan init-apply init-apply-auto

export AWS_PAGER :=

bootstrap-backend:
	@echo ">> checking/creating S3 bucket: $(BUCKET)"
	@aws s3api head-bucket --bucket $(BUCKET) --region $(REGION) 2>/dev/null && \
		echo "Bucket $(BUCKET) already exists" || \
		( aws s3api create-bucket \
			--bucket $(BUCKET) \
			--region $(REGION) \
			--create-bucket-configuration LocationConstraint=$(REGION) && \
		  echo "Created bucket $(BUCKET)" )
	@aws s3api put-bucket-versioning \
		--bucket $(BUCKET) \
		--versioning-configuration Status=Enabled
	@aws s3api put-bucket-encryption \
		--bucket $(BUCKET) \
		--server-side-encryption-configuration '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'
	@aws s3api put-public-access-block \
		--bucket $(BUCKET) \
		--public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true

destroy-backend:
	@echo ">> WARNING: this will permanently delete bucket $(BUCKET)"
	@if ! aws s3api head-bucket --bucket $(BUCKET) 2>/dev/null; then \
		echo ">> Bucket $(BUCKET) does not exist (or access is denied). Skipping deletion."; \
		exit 0; \
	fi; \
	echo ">> Emptying bucket (including all versions)"; \
	aws s3api list-object-versions --bucket $(BUCKET) --output json \
		--query '{Objects: Versions[].{Key:Key,VersionId:VersionId}}' > /tmp/_versions.json 2>/dev/null || echo '{"Objects":[]}' > /tmp/_versions.json; \
	count=$$(aws s3api list-object-versions --bucket $(BUCKET) --query 'length(Versions[])' --output text 2>/dev/null); \
	if [ "$$count" != "None" ] && [ "$$count" != "" ] && [ "$$count" -gt 0 ] 2>/dev/null; then \
		aws s3api delete-objects --bucket $(BUCKET) --delete file:///tmp/_versions.json; \
	fi; \
	aws s3api list-object-versions --bucket $(BUCKET) --output json \
		--query '{Objects: DeleteMarkers[].{Key:Key,VersionId:VersionId}}' > /tmp/_markers.json 2>/dev/null || echo '{"Objects":[]}' > /tmp/_markers.json; \
	mcount=$$(aws s3api list-object-versions --bucket $(BUCKET) --query 'length(DeleteMarkers[])' --output text 2>/dev/null); \
	if [ "$$mcount" != "None" ] && [ "$$mcount" != "" ] && [ "$$mcount" -gt 0 ] 2>/dev/null; then \
		aws s3api delete-objects --bucket $(BUCKET) --delete file:///tmp/_markers.json; \
	fi; \
	rm -f /tmp/_versions.json /tmp/_markers.json; \
	echo ">> Deleting bucket $(BUCKET)"; \
	aws s3api delete-bucket --bucket $(BUCKET) --region $(REGION)

init: bootstrap-backend
	terraform init

plan: bootstrap-backend
	terraform plan

apply: init
	terraform apply

apply-auto: init
	terraform apply --auto-approve

destroy: init
	terraform destroy
	$(MAKE) destroy-backend

destroy-auto: init
	terraform destroy --auto-approve
	$(MAKE) destroy-backend

fmt:
	terraform fmt -recursive

validate:
	terraform validate

init-plan: init plan

init-apply: init apply

init-apply-auto: init apply-auto

clean:
	rm -rf .terraform .terraform.lock.hcl
