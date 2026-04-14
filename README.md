# Gender Classify API

Small Go HTTP API that classifies likely gender for a given first name using the external [Genderize](https://api.genderize.io) service.

## Overview

- Runtime: Go `1.24.4`
- Default server address: `:8080`
- Exposed endpoint: `GET /api/classify`
- Upstream dependency: `https://api.genderize.io`

## How It Works

1. Validates query parameter `name`.
1. Calls Genderize with an HTTP client timeout.
1. Rejects requests when there is no usable prediction.
1. Returns normalized JSON including confidence metadata.

Confidence is derived with this rule:

`is_confident = probability >= 0.7 && sample_size >= 100`

## Run Locally

### Prerequisites

- Go `1.24.4`
- Internet access (required to call Genderize)

### Start the API

```bash
go run .
```

Expected startup log:

```text
server listening on :8080
```

## API Contract

### `GET /api/classify`

Classify likely gender for a single first name.

### Query Parameters

- `name` (required): single string value

Validation rules:

- Missing or empty `name` returns `400`.
- Multiple `name` values (for example `?name=a&name=b`) returns `422`.

### Success Response (`200`)

```json
{
	"status": "success",
	"data": {
		"name": "peter",
		"gender": "male",
		"probability": 0.99,
		"sample_size": 796,
		"is_confident": true,
		"processed_at": "2026-04-14T17:30:20Z"
	}
}
```

Field notes:

- `sample_size` maps directly from Genderize `count`.
- `processed_at` is UTC in RFC3339 format.

### Error Response Shape

All non-`200` responses use:

```json
{
	"status": "error",
	"message": "..."
}
```

### Error Cases

- `400 Bad Request`: `name` is missing or empty
	- Message: `name parameter is required`
- `405 Method Not Allowed`: method is not `GET`
	- Message: `method not allowed`
- `422 Unprocessable Entity`: `name` has multiple values
	- Message: `name must be a single string value`
- `422 Unprocessable Entity`: no prediction available (`gender` is null or `count` is 0)
	- Message: `No prediction available for the provided name`
- `502 Bad Gateway`: Genderize call failed or timed out
	- Message: `failed to reach gender prediction service`

## Example Requests

### Valid request

```bash
curl "http://localhost:8080/api/classify?name=peter"
```

### Missing name

```bash
curl "http://localhost:8080/api/classify"
```

### Multiple name values

```bash
curl "http://localhost:8080/api/classify?name=anna&name=elsa"
```

## Project Files

- `main.go`: HTTP server, request validation, upstream call, and response shaping
- `logic.txt`: decision flow summary for endpoint behavior
- `go.mod`: module metadata and Go version

## Notes

- CORS is currently open (`Access-Control-Allow-Origin: *`).
- Upstream response decoding expects JSON from Genderize.