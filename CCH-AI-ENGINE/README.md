# CCH AI Engine

Python FastAPI service boundary for AI inference. This scaffold only exposes liveness; no provider, model, prompt, customer-data policy, or database access is configured.

## Run

```powershell
python -m venv .venv
.venv\Scripts\Activate.ps1
python -m pip install -r requirements.txt
python -m uvicorn app.main:app --reload --host 127.0.0.1 --port 8000
```

`GET /healthz` returns the service identity and liveness status. Do not expose this service publicly; the Go backend will be its caller after service-to-service authentication is designed.

## Test

```powershell
python -m pytest
```