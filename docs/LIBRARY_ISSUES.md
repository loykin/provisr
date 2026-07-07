# Library Usability Issues

외부 프로젝트에서 `github.com/loykin/provisr`를 import해서 쓸 때 발생하는 문제 및 개선 사항 정리.

---

## 🔴 Critical — 지금 당장 외부에서 못 쓰는 것들

### 1. `NewTLSServer` 파라미터 타입 미노출

```go
// provisr.go:152
func NewTLSServer(serverConfig cfg.ServerConfig, m *Manager) (*http.Server, error)
```

`cfg.ServerConfig`는 `internal/config.ServerConfig`인데 `provisr.go`에 public alias가 없다.
외부 소비자는 이 타입을 이름으로 쓸 수 없어서 `NewTLSServer`를 호출할 방법이 없다.

**수정**: `provisr.go`에 추가

```go
type Config       = cfg.Config
type ServerConfig = cfg.ServerConfig
type TLSConfig    = cfg.TLSConfig
type AuthConfig   = cfg.AuthConfig
```

---

### 2. `LoadConfig` 반환 타입 미노출

```go
func LoadConfig(path string) (*cfg.Config, error)
```

반환값을 변수에 담을 수 있지만, 타입을 명시적으로 쓸 수 없다.
`ServerConfig`와 함께 위의 alias 추가로 해결된다.

---

### 3. Lifecycle Hook 타입 미노출

README와 예제에서 다음처럼 사용하도록 안내하지만 외부에서 `internal/process`를 import할 수 없다.

```go
// 현재 예제 코드 — 외부 소비자는 컴파일 에러
import "github.com/loykin/provisr/internal/process"

spec := provisr.Spec{
    Lifecycle: process.LifecycleHooks{   // ❌ 접근 불가
        PreStart: []process.Hook{...},
    },
}
```

`Spec` 자체는 `provisr.Spec`으로 노출되어 있지만, 그 안의 `Lifecycle` 필드 타입들이 없다.

**수정**: `provisr.go`에 추가

```go
type LifecycleHooks = process.LifecycleHooks
type Hook           = process.Hook
type FailureMode    = process.FailureMode
type RunMode        = process.RunMode
type LifecyclePhase = process.LifecyclePhase

const (
    FailureModeFail   = process.FailureModeFail
    FailureModeIgnore = process.FailureModeIgnore
    FailureModeRetry  = process.FailureModeRetry

    RunModeBlocking = process.RunModeBlocking
    RunModeAsync    = process.RunModeAsync
)
```

---

## 🟡 Medium — 사용은 되지만 문제가 있는 것들

### 4. 예제 13개가 `internal/` 패키지를 직접 import

외부 소비자가 예제를 참고해서 따라 치면 즉시 컴파일 에러가 난다.

영향받는 예제:
- `examples/embedded_http_gin/main.go` — `internal/manager`, `internal/process`, `internal/server`
- `examples/embedded_http_echo/main.go` — 동일
- `examples/embedded_http_gin_individual/main.go` — 동일
- `examples/embedded_http_echo_individual/main.go` — 동일
- `examples/embedded_lifecycle_hooks/main.go` — `internal/process`
- `examples/embedded_lifecycle_failure_modes/main.go` — `internal/process`
- `examples/embedded_job_lifecycle/main.go` — `internal/job`, `internal/process`
- `examples/embedded_lifecycle_config/main.go` — `internal/config`
- `examples/embedded_client/main.go` — `internal/logger`
- `examples/auth_basic/main.go` — `internal/auth`
- `examples/store_basic/main.go` — `internal/store`
- `examples/tls_example/main.go` — `internal/tls`
- `examples/unified_logging_demo/main.go` — `internal/logger`

**수정**: 각 예제를 public API(`provisr.*`)만 사용하도록 재작성.
그러려면 위 Critical 항목들(alias 추가)이 먼저 해결되어야 한다.

---

### 5. HTTP embed용 타입 미노출 (`server` 패키지)

Gin/Echo에 개별 핸들러를 붙이는 `APIEndpoints`가 `internal/server`에만 있고 public facade에 없다.

```go
// 현재는 이렇게만 가능
provisr.NewHTTPServer(addr, basePath, mgr)  // 서버 통째로 시작

// 이건 외부에서 못 함
endpoints := server.NewAPIEndpoints(mgr, "/api")  // ❌ internal
endpoints.RegisterAll(r.Group("/api"))
```

**수정**: `provisr.go`에 `APIEndpoints` 파사드 추가 또는 Gin/Echo embed용 helper 함수 추가.

---

### 6. Logger 설정 타입 미노출

`Spec.Log` 필드가 `logger.Config` 타입인데 이것도 `internal/logger`에 있다.
로그 디렉토리나 파일 경로를 코드에서 지정하려면 타입을 알아야 한다.

```go
spec := provisr.Spec{
    Log: ???,  // logger.Config 타입인데 외부에서 못 씀
}
```

**수정**: `type LogConfig = logger.Config` alias 추가.

---

## 🟠 의존성 문제

### 7. 불필요한 의존성이 전부 따라옴

라이브러리를 import하면 go.sum 기준으로 다음이 모두 포함된다:

| 패키지 | 크기 영향 | 실제 필요한 경우 |
|--------|-----------|-----------------|
| `gin` + `echo` | 큼 | HTTP 서버 embed 시에만 |
| `clickhouse-go/v2` | 매우 큼 | History를 ClickHouse에 저장할 때만 |
| `mongo-driver/v2` | 큼 | 현재 코드에서 실제로 쓰는지 불명확 |
| `pgx/v5` + `lib/pq` | 중간 | PostgreSQL 쓸 때만 |
| `modernc.org/sqlite` | 큼 | SQLite 쓸 때만 |
| `prometheus/client_golang` | 중간 | 메트릭 쓸 때만 |
| `golang-jwt/jwt` | 작음 | 인증 쓸 때만 |

**개선 방향**: 기능별로 sub-module 분리 또는 build tag로 선택적 컴파일.

```
github.com/loykin/provisr           — 코어 (Manager, Job, CronJob)
github.com/loykin/provisr/http      — HTTP 서버 (gin/echo 의존)
github.com/loykin/provisr/metrics   — Prometheus
github.com/loykin/provisr/history   — ClickHouse/PostgreSQL/SQLite
github.com/loykin/provisr/auth      — JWT/인증
```

---

## 🟢 추가하면 좋은 것들

### 8. 프로세스 준비 상태(Readiness) 감지 개선

현재 `Detector` 인터페이스가 있지만 외부에서 커스텀 detector를 등록하는 방법이 공개 API에 없다.
예: "포트 8888이 열릴 때까지 기다린 후 started로 판단" 같은 로직.

**추가**: `RegisterDetector` 또는 `WithDetector(func() bool)` 같은 public API.

---

### 9. 파이프라인용 의존성 그래프 (DAG) 지원

현재 Job들은 독립적으로 실행된다. "A Job이 완료된 후 B Job 시작" 같은 의존 관계가 없다.

단순한 형태:

```go
jobMgr.CreateJob(provisr.JobSpec{
    Name:    "stage-b",
    Command: "python stage_b.py",
    DependsOn: []string{"stage-a"},  // 추가 필요
})
```

---

### 10. 프로세스 출력(stdout/stderr) 스트리밍 API

현재는 로그 파일에 기록하는 방식만 있다.
외부 라이브러리 소비자 입장에서 프로세스 출력을 실시간으로 받을 수 있는 API가 없다.

```go
// 이런 API가 있으면 유용
reader, err := mgr.OutputReader("my-process")  // io.Reader 반환
```

---

### 11. Graceful shutdown API

`Manager` 전체를 한 번에 종료하는 public API가 없다.
서버 애플리케이션에서 SIGTERM 받았을 때 모든 managed process를 정리하고 싶을 때 필요.

```go
// 현재: 개별 StopAll만 있음
mgr.StopAll("", 10*time.Second)

// 필요한 것: 컨텍스트 기반 전체 종료
mgr.Shutdown(ctx)
```

---

## 작업 우선순위 요약

| 우선순위 | 항목 | 상태 |
|----------|------|------|
| P0 | `Config`, `ServerConfig`, `TLSConfig`, `AutoGenTLS`, `ServerAuthConfig` alias 추가 | ✅ 완료 |
| P0 | `LifecycleHooks`, `Hook`, `FailureMode`, `RunMode`, `LifecyclePhase` alias + 상수 추가 | ✅ 완료 |
| P0 | `LogConfig`, `LogFileConfig`, `LogSlogConfig` alias 추가 | ✅ 완료 |
| P1 | 예제 13개 internal import 제거 | ✅ 완료 |
| P1 | `Router`, `APIEndpoints` public facade 추가 | ✅ 완료 |
| P1 | `pkg/logger`, `pkg/auth`, `pkg/store`, `pkg/tls` 공개 패키지 신설 | ✅ 완료 |
| P3 | `Detector`, `CommandDetector` interface/타입 노출 | ✅ 완료 |
| P3 | stdout/stderr 스트리밍 (`Spec.Log.File.StdoutWriter`/`StderrWriter`) | ✅ 완료 |
| P3 | `Manager.Shutdown()` API | ✅ 완료 |
| P2 | 의존성 분리 (sub-module 또는 build tag) | ✅ 완료 |
| P4 | DAG 의존성 지원 | ✅ 완료 |
