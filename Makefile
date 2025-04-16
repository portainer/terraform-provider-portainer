.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@echo "  build                  Compile the Terraform provider binary"
	@echo "  install-plugin         Install compiled provider binary to local Terraform plugin directory"
	@echo "  init                   Initialize Terraform in each examples/* project"
	@echo "  validate               Validate Terraform configuration in each project"
	@echo "  fmt-check              Check formatting of Terraform files"
	@echo "  fmt                    Format Terraform files"
	@echo "  docs                   Generate terraform-docs in each project (if main.tf exists)"
	@echo "  o-init                 Initialize OpenTofu in each examples/* project"
	@echo "  o-validate             Validate OpenTofu configuration"
	@echo "  o-fmt-check            Check formatting of OpenTofu files"
	@echo "  o-fmt                  Format OpenTofu files"
	@echo "  up                     Start Docker Compose services"
	@echo "  launch                 Open https://localhost:9000 in default browser"
	@echo "  down                   Stop Docker Compose services"
	@echo "  up-agent               Start Portainer Agent via docker-compose.agent.yml"
	@echo "  down-agent             Stop Portainer Agent"
	@echo "  install-k3d            Install k3d CLI"
	@echo "  k3d-up                 Create local K3d cluster with 1 agent"
	@echo "  k3d-status             Show Kubernetes nodes"
	@echo "  k8s-deploy-agent       Deploy Portainer agent into K8s via LoadBalancer"
	@echo "  k3d-connect-portainer  Connect Portainer container to k3d network"
	@echo "  k3d-export-ip          Extract K3d IP and save terraform.tfvars"
	@echo "  install-kubectl        Install latest kubectl CLI (auto-detects OS/arch)"
	@echo "  go-fmt-check           Check formatting of Go source files"
	@echo "  go-fmt                 Format Go source files"
	@echo ""
	@echo "Environment:"
	@echo "  TDIR                   Directory to run Terraform/OpenTofu in (set internally)"
	@echo "  TCMD                   Terraform/OpenTofu command (init, validate, fmt, etc.)"
	@echo ""

### Terraform
.PHONY: build
build:
	go build -o terraform-provider-portainer

.PHONY: install-plugin
install-plugin:
	mkdir -p ~/.terraform.d/plugins/localdomain/local/portainer/0.1.0/linux_amd64/
	cp terraform-provider-portainer ~/.terraform.d/plugins/localdomain/local/portainer/0.1.0/linux_amd64/

.PHONY: init
init:
	@for project in $$(find examples -type d -mindepth 1 -maxdepth 1); do \
		$(MAKE) _terraform TDIR=$$project TCMD=init ; \
	done

.PHONY: validate
validate:
	@for project in $$(find examples -type d -mindepth 1 -maxdepth 1); do \
		$(MAKE) _terraform TDIR=$$project TCMD=validate ; \
	done

.PHONY: fmt-check
fmt-check:
	@for project in $$(find examples -type d -mindepth 1 -maxdepth 1); do \
		$(MAKE) _terraform TDIR=$$project TCMD="fmt -check" ; \
	done

.PHONY: fmt
fmt:
	@for project in $$(find examples -type d -mindepth 1 -maxdepth 1); do \
		$(MAKE) _terraform TDIR=$$project TCMD="fmt" ; \
	done
	@for project in $$(find e2e-tests -type d -mindepth 1 -maxdepth 1); do \
		$(MAKE) _terraform TDIR=$$project TCMD="fmt" ; \
	done

_terraform:
	terraform -chdir=${TDIR} ${TCMD}

### DOCS
.PHONY: docs
docs:
	@for dir in $$(find examples -type d -mindepth 1 -maxdepth 1); do \
		if [ -f $$dir/main.tf ]; then \
			terraform-docs -c .terraform-docs.yml $$dir; \
		fi; \
	done
	@for dir in $$(find e2e-tests -type d -mindepth 1 -maxdepth 1); do \
		if [ -f $$dir/main.tf ]; then \
			terraform-docs -c .terraform-docs.yml $$dir; \
		fi; \
	done

### Opentofu
.PHONY: o-init
o-init:
	@for project in $$(find examples -type d -mindepth 1 -maxdepth 1); do \
		$(MAKE) _opentofu TDIR=$$project TCMD=init ; \
	done

.PHONY: o-validate
o-validate:
	@for project in $$(find examples -type d -mindepth 1 -maxdepth 1); do \
		$(MAKE) _opentofu TDIR=$$project TCMD=validate ; \
	done

.PHONY: o-fmt-check
o-fmt-check:
	@for project in $$(find examples -type d -mindepth 1 -maxdepth 1); do \
		$(MAKE) _opentofu TDIR=$$project TCMD="fmt -check" ; \
	done

.PHONY: o-fmt
o-fmt:
	@for project in $$(find examples -type d -mindepth 1 -maxdepth 1); do \
		$(MAKE) _opentofu TDIR=$$project TCMD="fmt" ; \
	done

_opentofu:
	tofu -chdir=${TDIR} ${TCMD}

### Docker
.PHONY: up
up:
	docker compose up -d

.PHONY: launch
launch:
	@PORTAINER_HOST=$${PORTAINER_HOST:-'localhost:9000'} ; \
	URL=$${URL:-http://$${PORTAINER_HOST}} ; \
	echo "Opening $${URL} ..." ; \
	OS=$$(uname | tr '[:upper:]' '[:lower:]') ; \
	if [ "$$OS" = "linux" ]; then \
		xdg-open "$${URL}" >/dev/null 2>&1 || echo "Could not open browser (xdg-open not found?)" ; \
	elif [ "$$OS" = "darwin" ]; then \
		open "$${URL}" ; \
	elif echo "$$OS" | grep -q "mingw\\|msys\\|cygwin"; then \
		start "$${URL}" ; \
	else \
		echo "Cannot open browser automatically on this OS: $$OS" ; \
	fi

.PHONY: down
down:
	docker compose down

.PHONY: up-agent
up-agent:
	docker compose -f docker-compose.agent.yml up -d

.PHONY: down-agent
down-agent:
	docker compose -f docker-compose.agent.yml down

### Kubernetes / k3d
.PHONY: install-k3d
install-k3d:
	curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

.PHONY: k3d-up
k3d-up:
	k3d cluster create mycluster --agents 1 -p "9001:32194@agent:0"

.PHONY: k3d-status
k3d-status:
	kubectl get nodes

.PHONY: k8s-deploy-agent
k8s-deploy-agent:
	kubectl apply -f https://downloads.portainer.io/ce2-27/portainer-agent-k8s-lb.yaml
	kubectl -n portainer wait --for=condition=available deployment/portainer-agent --timeout=120s

.PHONY: k3d-connect-portainer
k3d-connect-portainer:
	docker network connect k3d-mycluster portainer || true

.PHONY: k3d-export-ip
k3d-export-ip:
	@IP=$$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' k3d-mycluster-server-0); \
	echo "💡 K3D Server IP: $$IP"; \
	echo "portainer_environment_address = \"tcp://$$IP:9001\"" > e2e-tests/environment/terraform.tfvars

.PHONY: install-kubectl
install-kubectl:
	@echo "🔍 Detecting platform and architecture..."
	@ARCH=$$(uname -m); \
	case $$ARCH in \
	  x86_64) ARCH=amd64 ;; \
	  arm64|aarch64) ARCH=arm64 ;; \
	  *) echo "❌ Unsupported architecture: $$ARCH" && exit 1 ;; \
	esac; \
	OS=$$(uname | tr '[:upper:]' '[:lower:]'); \
	if [ "$$OS" != "linux" ] && [ "$$OS" != "darwin" ]; then \
		echo "❌ Unsupported OS: $$OS"; exit 1; \
	fi; \
	VERSION=$$(curl -L -s https://dl.k8s.io/release/stable.txt); \
	URL="https://dl.k8s.io/release/$$VERSION/bin/$$OS/$$ARCH/kubectl"; \
	echo "⬇️ Downloading kubectl $$VERSION for $$OS/$$ARCH..."; \
	curl -LO "$$URL"; \
	chmod +x kubectl; \
	install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl; \
	rm -f kubectl; \
	echo "✅ kubectl installed to /usr/local/bin"

### Go
.PHONY: go-fmt-check
go-fmt-check:
	@echo "Checking Go code formatting..."
	@unformatted=$$(find . -type f -name '*.go' -not -path './portainer_data/*' 2>/dev/null | xargs gofmt -s -l); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not properly formatted:"; \
		echo "$$unformatted"; \
		echo ""; \
		echo "Run 'make go-fmt' to format them."; \
		exit 1; \
	else \
		echo "All Go files are properly formatted."; \
	fi

.PHONY: go-fmt
go-fmt:
	@echo "Formatting Go code..."
	@find . -type f -name '*.go' -not -path './portainer_data/*' 2>/dev/null | xargs gofmt -s -w
	@echo "Done."
