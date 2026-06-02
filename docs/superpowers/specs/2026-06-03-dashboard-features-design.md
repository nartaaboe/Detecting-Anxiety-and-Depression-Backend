# Dashboard Features Design

**Date:** 2026-06-03  
**Scope:** Audit logging, WebSocket admin notifications, PDF export, risk trend chart

---

## 1. Overview

Four features are added to the existing mental health detection platform:

| Feature                       | Where                                                     |
| ----------------------------- | --------------------------------------------------------- |
| High-risk audit logging       | Backend worker → `audit_logs` table                       |
| Real-time admin notifications | Go WebSocket hub → React admin panel                      |
| PDF export                    | Backend data endpoint + frontend `jsPDF` render           |
| Risk dynamics chart           | New trend endpoint + `recharts` on Dashboard & AdminPanel |

Email notifications are out of scope for this iteration.

---

## 2. High-Risk Audit Logging

### What

When the worker processes an analysis and the AI returns `label == "high"`, write a row to the existing `audit_logs` table.

### How

The `AuditRepo.Create()` method already exists. Add `*AuditRepo` to `WorkerPool` struct and call it in `pool.go` after a successful `results.Upsert()`.

```
action      = "high_risk_detected"
entity_type = "analysis"
entity_id   = analysis.ID
actor_user_id = analysis.UserID
ip          = "" (worker has no IP context)
meta_json   = {"score": 0.95, "confidence": 0.80, "model_version": "mentalbert-v1", "label": "high"}
```

### Admin visibility

The existing admin audit log endpoint (`GET /admin/audit-logs`) already returns all logs. Add a query param filter `?action=high_risk_detected` so the admin panel can show only high-risk events.

No new DB table needed — `audit_logs` already has the right schema and indexes.

---

## 3. WebSocket Admin Notifications

### Architecture

```
Worker detects high-risk
        │
        ▼
   Hub.Broadcast(msg)
        │
   ┌────┴────┐
   │  Hub    │  (singleton, runs in background goroutine)
   └────┬────┘
        │  fan-out to all connected admin clients
   ┌────┴────┐
   │ Client  │──► WebSocket conn ──► Admin browser
   └─────────┘
```

### Hub (`internal/ws/hub.go`)

```go
type Hub struct {
    clients    map[*Client]bool
    broadcast  chan []byte
    register   chan *Client
    unregister chan *Client
}

type Client struct {
    hub  *Hub
    conn *websocket.Conn
    send chan []byte
}
```

Hub runs a single `Run()` goroutine that handles register/unregister/broadcast. Each `Client` runs a `writePump()` goroutine that drains `send` channel to the WebSocket.

### Endpoint

```
GET /ws/admin/notifications?token=<jwt>
```

JWT is passed as query param because WebSocket handshake cannot set `Authorization` headers from the browser. The handler validates the token, checks the user has admin role, then upgrades to WebSocket and registers the client with the hub.

### Message format (JSON)

```json
{
  "type": "high_risk_alert",
  "analysis_id": "uuid",
  "user_id": "uuid",
  "label": "high",
  "score": 0.95,
  "confidence": 0.8,
  "model_version": "mentalbert-v1",
  "at": "2026-06-03T03:10:00Z"
}
```

### Frontend (`src/ws/useAdminNotifications.ts`)

React hook that:

1. Reads JWT from auth context, opens `new WebSocket(url?token=jwt)`
2. On message: appends to local `notifications[]` state, shows `Toast`
3. Maintains `unreadCount` badge on admin nav item
4. Reconnects with exponential backoff on disconnect

---

## 4. PDF Export

### Backend (`GET /analyses/export`)

Returns a single JSON payload with everything needed to render the report:

```json
{
  "user":     { "email": "...", "created_at": "..." },
  "stats":    { "total": 10, "high_risk": 2, "medium_risk": 3, "low_risk": 5, "avg_confidence": 0.81 },
  "trend":    { "avg_7d": 0.62, "avg_30d": 0.50, "delta": 0.12, "count_7d": 4, "count_30d": 10 },
  "analyses": [
    {
      "id": "...", "text": "...", "label": "high",
      "score": 0.95, "confidence": 0.80,
      "model_version": "mentalbert-v1",
      "created_at": "...",
      "explanation": { "key_phrases": [...], "top_sentences": [...] }
    }
  ],
  "exported_at": "2026-06-03T03:00:00Z"
}
```

Reuses existing `DashboardService.Stats()`, `DashboardService.Summary()`, and analyses list queries. No new DB work.

### Frontend

**`ExportReport.tsx`** — a hidden, A4-styled React component. Rendered off-screen with fetched data. Contains:

- Header: user email + export date
- Summary cards: total, high/medium/low counts, avg confidence
- Trend block: avg 7d vs 30d
- Table: all analyses (date, label, score, confidence, model, first 200 chars of text, key phrases)

**Flow when user clicks "Экспорт PDF":**

1. `GET /analyses/export` → data
2. Render `ExportReport` into a hidden `div` (via `createPortal`)
3. `html2canvas(div)` → canvas
4. `jsPDF` adds canvas as image, saves `report-<date>.pdf`
5. Hidden div unmounts

**Libraries to add:** `jspdf`, `html2canvas`

---

## 5. Risk Dynamics Chart

### Backend

New method `TrendPoints(userID, period, n)` in `DashboardService`:

```sql
-- period = 'week', n = 12
SELECT
  date_trunc('week', ar.created_at)             AS period,
  ROUND(AVG(ar.score)::numeric, 4)              AS avg_score,
  COUNT(*)                                       AS total,
  COUNT(*) FILTER (WHERE ar.label = 'high')     AS high_count,
  COUNT(*) FILTER (WHERE ar.label = 'medium')   AS medium_count,
  COUNT(*) FILTER (WHERE ar.label = 'low')      AS low_count
FROM analysis_results ar
JOIN analyses a ON a.id = ar.analysis_id
WHERE a.user_id = $1
  AND ar.created_at >= now() - ($2 || ' weeks')::interval
GROUP BY period
ORDER BY period ASC
```

Admin variant omits the `user_id` filter.

**New endpoints:**

```
GET /dashboard/trend?period=weekly&n=12    — current user
GET /admin/trend?period=weekly&n=12        — all users (admin only)
```

Response:

```json
[
  {
    "period": "2026-03-17",
    "avg_score": 0.45,
    "total": 5,
    "high": 1,
    "medium": 2,
    "low": 2
  },
  {
    "period": "2026-03-24",
    "avg_score": 0.61,
    "total": 3,
    "high": 2,
    "medium": 1,
    "low": 0
  }
]
```

### Frontend

**Library:** `recharts` (add to package.json)

**`RiskTrendChart.tsx`** — reusable component:

- `ComposedChart` with `Bar` (stacked: high=red, medium=amber, low=green) + `Line` (avg_score, right Y-axis)
- Period selector toggle: "По неделям" / "По месяцам"
- Responsive via `ResponsiveContainer`

**Placement:**

- `Dashboard.tsx` — shows current user's chart below existing stats cards
- `AdminPanel.tsx` — shows global chart in a dedicated tab/section

---

## 6. Build Order

1. **Audit logging** — smallest change, just wires existing code in worker
2. **Trend chart backend** — new SQL + endpoints, no external deps
3. **Trend chart frontend** — add recharts, build RiskTrendChart component
4. **WebSocket hub** — new internal package, wire into router and worker
5. **WebSocket frontend** — hook + admin panel UI
6. **PDF export backend** — new endpoint, reuses existing services
7. **PDF export frontend** — add jspdf + html2canvas, build ExportReport

---

## 7. New Files

**Backend:**

- `internal/ws/hub.go`
- `internal/ws/client.go`

**Frontend:**

- `src/ws/useAdminNotifications.ts`
- `src/components/RiskTrendChart.tsx`
- `src/components/ExportReport.tsx`

**Modified:**

- `internal/workers/pool.go` — audit log on high-risk
- `internal/services/dashboard.go` — TrendPoints()
- `internal/http/handlers.go` — 4 new handlers
- `internal/http/router.go` — 4 new routes
- `go.mod` / `go.sum` — gorilla/websocket
- `package.json` — recharts, jspdf, html2canvas
- `src/pages/Dashboard.tsx` — chart section
- `src/pages/AdminPanel.tsx` — WS notifications + global chart
- `src/pages/History.tsx` — export button
