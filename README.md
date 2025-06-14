# OpenTelemetry Example with Gin, SQLite, Tracing, and Metrics

This repository demonstrates how to instrument a simple Go web service using:

- [Gin](https://github.com/gin-gonic/gin) HTTP framework
- [SQLite](https://github.com/mattn/go-sqlite3)
- [OpenTelemetry](https://opentelemetry.io/) for tracing and metrics
- [XSAM/otelsql](https://github.com/XSAM/otelsql) for SQL tracing and metrics
- Configurable tracing backend: **stdout** or **Jaeger**

## Features

- HTTP tracing and metrics with OpenTelemetry
- Instrumented SQL queries with `otelsql`
- Custom metrics: request count and DB query durations
- Choice between stdout tracing and Jaeger
- In-memory SQLite database with a simple `users` table

## Requirements

- Go 1.21+
- (Optional) Docker (for running Jaeger locally)

## Getting Started

### 1. Clone the Repo

```bash
git clone https://github.com/yourusername/opentelemetry-gin-sqlite.git
cd opentelemetry-gin-sqlite
````

### 2. Run with Stdout Tracing

```bash
# you may either use go run
go run main.go --trace=stdout

# or you may compile code first and run it 
go build
./tracer -trace=stdout
```

You’ll see spans and metrics printed to the console.


### 3. Run with Jaeger Tracing
For jaeger setup you have two options:
- start jaeger server using its executable, see its download
  [page](https://www.jaegertracing.io/download/). In this case you simply start
  the executable `/path/to/jaeger`
- or, start Jaeger using Docker:

```bash
docker run -d --name jaeger \
  -e COLLECTOR_ZIPKIN_HOST_PORT=:9411 \
  -p 16686:16686 -p 14268:14268 \
  jaegertracing/all-in-one:1.54
```

Then start the app:

```bash
# via run command
go run main.go --trace=jaeger

# via built executable
go build
./tracer -trace=jaeger
```

Open [http://localhost:16686](http://localhost:16686) to explore traces.

### 4. Try the API

Use `curl` or browser to test:

```bash
curl http://localhost:8080/user/1
```

Response:

```json
{"id":"1","name":"Alice"}
```

To simulate the load you may user
[hey](https://github.com/rakyll/hey) tool and run it as simple as
```
hey http://localhost:8080/user/1
```
This will simulate load of 200 requests to your server.

Here are a few screen shots of executed workflow in jaeger:
![image1](images/tracing_jaeger1.png)
![image2](images/tracing_jaeger2.png)
![image3](images/tracing_jaeger3.png)

### 5. View Metrics

Metrics are printed to stdout periodically (e.g., every 60s). Look for:

* `http_requests_total`
* `db_query_duration_ms`
* DB stats via `RegisterDBStatsMetrics`

## Project Structure

* `main.go`: Core application logic and instrumentation
* Flags:

  * `--trace=stdout` (default)
  * `--trace=jaeger`

## TODOs / Extensions

* Export metrics to Prometheus
* Add more endpoints and richer tracing
* Support other databases (e.g., Postgres, MySQL)
* Add logging with context-aware span injection

## License

MIT

---

Built for educational and demonstration purposes.
