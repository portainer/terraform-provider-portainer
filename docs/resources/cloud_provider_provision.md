# ☁️ Resource Documentation: `portainer_cloud_provider_provision`

## Overview
The `portainer_cloud_provider_provision` resource provisions a new Kubernetes cluster using supported cloud providers through the Portainer API.

> Currently working only for Portainer BE edition

It currently supports:
- Civo, DigitalOcean, Linode → via `/cloud/{provider}/provision`
- Amazon EKS → via `/cloud/amazon/provision`
- Azure AKS → via `/cloud/azure/provision`

---

## 📘 Example Usage

### 🌐 DigitalOcean (Civo, Linode)
```hcl
resource "portainer_cloud_provider_provision" "do_cluster" {
  cloud_provider = "digitalocean"

  payload = {
    credentialID       = 1
    name               = "do-dev-cluster"
    region             = "nyc1"
    nodeCount          = 3
    nodeSize           = "s-2vcpu-4gb"
    networkID          = "1234-abcd"
    kubernetesVersion  = "1.25.0"
    meta = jsonencode({
      customTemplateID     = 0
      groupId              = 1
      stackName            = "dev"
      tagIds               = [1]
    })
  }
}
```
### ☁️ Amazon (EKS)
```hcl
resource "portainer_cloud_provider_provision" "eks_cluster" {
  cloud_provider = "amazon"

  payload = {
    credentialID       = 1
    name               = "eks-dev"
    region             = "us-east-1"
    nodeCount          = 3
    instanceType       = "m5.large"
    amiType            = "BOTTLEROCKET_x86_64"
    nodeVolumeSize     = 20
    networkID          = "vpc-abc123"
    kubernetesVersion  = "1.24"
    meta = jsonencode({
      groupId          = 1
      stackName        = "eks"
      tagIds           = [1]
    })
  }
}
```

### ☁️ Azure (AKS)
```hcl
resource "portainer_cloud_provider_provision" "aks_cluster" {
  cloud_provider = "azure"

  payload = {
    credentialID        = 1
    name                = "aks-dev"
    region              = "eastus"
    nodeCount           = 3
    nodeSize            = "Standard_DS2_v2"
    networkID           = "vnet-subnet"
    dnsPrefix           = "aksdns"
    resourceGroup       = "rg-dev"
    resourceGroupName   = "rg-dev"
    poolName            = "default"
    tier                = "standard"
    kubernetesVersion   = "1.24.0"
    availabilityZones   = ["1", "2"]
    meta = jsonencode({
      groupId           = 1
      stackName         = "aks"
      tagIds            = [1]
    })
  }
}
```

### ☸️ Google Kubernetes Engine (GKE)
```hcl
resource "portainer_cloud_provider_provision" "gke_cluster" {
  cloud_provider = "gke"

  payload = {
    credentialID       = 1
    name               = "gke-dev"
    region             = "us-central1"
    nodeCount          = 3
    nodeSize           = "e2-standard-2"
    cpu                = 2
    ram                = 4
    hdd                = 100
    networkID          = "gke-vpc"
    kubernetesVersion  = "1.25"
    meta = jsonencode({
      groupId          = 1
      stackName        = "gke"
      tagIds           = [1]
    })
  }
}
```
---

## ⚙️ Lifecycle Behavior

---

## 🧾 Arguments Reference

### Top-Level
| Name      | Type   | Required | Description                                |
|-----------|--------|----------|--------------------------------------------|
| `cloud_provider`| string | ✅ yes   | One of `civo`, `digitalocean`, `linode`, `amazon`, `azure` |
| `payload` | map    | ✅ yes   | Provisioning details (see per-provider table) |

---

## 🌐 Civo, DigitalOcean, Linode - Payload Fields
| Name               | Type     | Required | Description                      |
|--------------------|----------|----------|----------------------------------|
| `credentialID`     | number   | ✅ yes   | ID of the Portainer cloud credential |
| `name`             | string   | ✅ yes   | Cluster name                     |
| `region`           | string   | ✅ yes   | Region (e.g. `NYC1`)             |
| `nodeCount`        | number   | ✅ yes   | Number of nodes                  |
| `nodeSize`         | string   | ✅ yes   | Node instance size               |
| `networkID`        | string   | ✅ yes   | Network ID or UUID               |
| `kubernetesVersion`| string   | ✅ yes   | Kubernetes version               |
| `meta`             | object   | 🚫 no    | Cluster metadata (template, groupId, tagIds) |

---

## ☁️ Amazon (EKS) - Payload Fields
| Name               | Type     | Required | Description                      |
|--------------------|----------|----------|----------------------------------|
| `credentialID`     | number   | ✅ yes   | ID of the Portainer cloud credential |
| `name`             | string   | ✅ yes   | Cluster name                     |
| `region`           | string   | ✅ yes   | AWS region                       |
| `nodeCount`        | number   | ✅ yes   | Number of nodes                  |
| `instanceType`     | string   | ✅ yes   | EC2 instance type                |
| `amiType`          | string   | 🚫 no    | AMI type (e.g. `BOTTLEROCKET_x86_64`) |
| `nodeVolumeSize`   | number   | 🚫 no    | Volume size in GB                |
| `networkID`        | string   | ✅ yes   | VPC ID                           |
| `kubernetesVersion`| string   | ✅ yes   | Kubernetes version               |
| `meta`             | object   | 🚫 no    | Metadata block                   |

---

## ☁️ Azure (AKS) - Payload Fields
| Name                  | Type     | Required | Description                      |
|-----------------------|----------|----------|----------------------------------|
| `credentialID`        | number   | ✅ yes   | ID of the Portainer cloud credential |
| `name`                | string   | ✅ yes   | Cluster name                     |
| `region`              | string   | ✅ yes   | Azure region                     |
| `nodeCount`           | number   | ✅ yes   | Number of nodes                  |
| `nodeSize`            | string   | ✅ yes   | Azure VM size                    |
| `networkID`           | string   | ✅ yes   | Network ID or subnet             |
| `dnsPrefix`           | string   | ✅ yes   | DNS prefix                       |
| `resourceGroup`       | string   | ✅ yes   | Azure Resource Group             |
| `resourceGroupName`   | string   | 🚫 no    | Optional separate resource group name |
| `poolName`            | string   | 🚫 no    | Pool name                        |
| `tier`                | string   | 🚫 no    | AKS service tier                 |
| `availabilityZones`   | list     | 🚫 no    | List of availability zones       |
| `kubernetesVersion`   | string   | ✅ yes   | Kubernetes version               |
| `meta`                | object   | 🚫 no    | Metadata                         |

---

## ☸️ Google GKE - Payload Fields
| Name               | Type     | Required | Description                      |
|--------------------|----------|----------|----------------------------------|
| `credentialID`     | number   | ✅ yes   | ID of the Portainer cloud credential |
| `name`             | string   | ✅ yes   | Cluster name                     |
| `region`           | string   | ✅ yes   | GCP region                       |
| `nodeCount`        | number   | ✅ yes   | Number of nodes                  |
| `nodeSize`         | string   | ✅ yes   | GCP machine type (e.g. `e2-medium`) |
| `networkID`        | string   | ✅ yes   | VPC network ID or name           |
| `kubernetesVersion`| string   | ✅ yes   | Kubernetes version               |
| `cpu`              | number   | 🚫 no    | Number of vCPUs per node         |
| `ram`              | number   | 🚫 no    | RAM in GB                        |
| `hdd`              | number   | 🚫 no    | HDD size in GB                   |
| `meta`             | object   | 🚫 no    | Metadata block                   |
