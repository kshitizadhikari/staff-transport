# Infrastructure

This directory holds deployment and environment infrastructure. The local
development stack lives in the repository-root `docker-compose.dev.yml`.

## Deployment Target

```text
Bitbucket
   ↓
Azure DevOps Pipeline
   ↓
Docker build
   ↓
Amazon ECR
   ↓
AWS runtime (ECS/Fargate or EC2)
```

Application containers must stay stateless. State belongs in PostgreSQL,
Redis, and object storage.

## Expected Contents

- Dockerfiles for the API and worker images
- Azure DevOps pipeline definitions
- AWS task/service definitions or Terraform
- Environment/secret configuration templates (no real secrets)

Nothing in this directory should contain committed credentials.
