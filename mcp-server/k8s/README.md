# Create the secret before deploying:
#
#   kubectl -n rag-etl create secret generic memorix-mcp-secret \
#     --from-literal=tenant-id=<your-tenant-id>
#
# Then deploy:
#
#   kubectl apply -f k8s/deployment.yaml
