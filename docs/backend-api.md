# Log upload backend

The Go service stores upload metadata and parsing results in PostgreSQL. Original and extracted files are stored under `LOG_STORAGE_DIR` (default: `data/logs`). Database migrations run automatically when the service starts.

## Start locally

Create an empty PostgreSQL database with the native PostgreSQL tools. The command prompts for the PostgreSQL administrator password:

```powershell
& "C:\Program Files\PostgreSQL\17\bin\createdb.exe" -U postgres -W logmaster
$encodedPassword = [uri]::EscapeDataString("YOUR_POSTGRES_PASSWORD")
$env:DATABASE_URL="postgres://postgres:$encodedPassword@127.0.0.1:5432/logmaster?sslmode=disable"
npm.cmd --prefix frontend run build
go run .
```

Replace the username and password in `DATABASE_URL` with the actual PostgreSQL credentials. The backend applies all tables and indexes automatically, serves the built Vue application at `/`, and exposes backend routes under `/api`.

## Upload logs

`POST /api/logs/upload` accepts `multipart/form-data`:

- `file`: one or more LOG, TXT, OUT, CSV, ZIP, GZ, TGZ, or TAR.GZ files
- `project_name`: optional project name, defaults to `default`
- `version`: optional firmware or software version

ZIP archives can be unencrypted, ZipCrypto-encrypted, or AES-encrypted. Encrypted ZIP entries use `70M_dashcam_^` as the default password. Archive paths and extracted sizes are validated before files are written.

Example response (`202 Accepted`):

```json
{
  "code": 0,
  "message": "upload accepted",
  "data": {
    "upload_id": "a UUID",
    "task_id": "a UUID",
    "status": "queued",
    "file_count": 3
  }
}
```

## Query APIs

- `GET /api/logs?page=1&page_size=20`: list uploads
- `GET /api/logs/{upload_id}`: upload details and extracted file list
- `GET /api/tasks?page=1&page_size=20`: list parsing tasks
- `GET /api/tasks/{task_id}`: parsing status and file statistics
- `GET /api/tasks/{task_id}/results?page=1&page_size=20`: matched error and warning lines
- `GET /api/tasks/{task_id}/agent-results`: optional Agent diagnoses grouped by log file

All backend routes use the `/api` prefix so they do not conflict with Vue Router paths served by the single-process frontend.

## Agent analysis extension

Set `AGENT_ANALYSIS_URL` to enable an external Agent. The backend sends one request after each log file completes local parsing. Agent failures are stored separately and do not fail the local parsing task.

Optional configuration:

- `AGENT_ANALYSIS_URL`: full HTTP endpoint receiving analysis requests
- `AGENT_ANALYSIS_TOKEN`: bearer token sent in the `Authorization` header
- `AGENT_ANALYSIS_TIMEOUT_SECONDS`: request timeout, default `60`

Request contract:

```json
{
  "task_id": "UUID",
  "upload_id": "UUID",
  "file": {
    "id": 1,
    "relative_path": "items/1/extracted/system.log",
    "size_bytes": 1024,
    "sha256": "...",
    "line_count": 200
  },
  "total_lines": 200,
  "matches": [
    {
      "level": "error",
      "matched_text": "ERROR",
      "line_number": 42,
      "content": "ERROR recorder failed",
      "file_path": ""
    }
  ]
}
```

Expected response contract:

```json
{
  "summary": "Recorder initialization failed",
  "findings": [
    {
      "category": "recording",
      "severity": "error",
      "root_cause": "Camera initialization timed out",
      "suggestion": "Check the camera connection and initialization sequence",
      "evidence": "ERROR recorder failed",
      "confidence": 0.92
    }
  ]
}
```

The Go integration point is the `AgentAnalyzer` interface in `internal/logs/agent.go`. Additional in-process Agent implementations can be injected with `NewServiceWithAgent` without changing upload or task APIs.
