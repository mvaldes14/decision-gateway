# n8n example

Import `task-triage.json` into n8n, then update the **Execute Decision** node URL for your deployment.

Default URL assumes n8n and Decision Gateway share a container or Kubernetes network:

```text
http://decision-gateway:8080/v1/decisions/task-triage/versions/v1:execute
```

Run the workflow manually. **Task Input** supplies the decision payload; **Execute Decision** returns the selected project, effort, scores, provider, and request ID.
