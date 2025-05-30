package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

var (
	db        *sql.DB
	meter     metric.Meter
	reqCount  metric.Int64Counter
	queryTime metric.Float64Histogram
	traceMode string
)

func main() {
	flag.StringVar(&traceMode, "trace", "stdout", "trace mode: stdout | jaeger")
	flag.Parse()

	ctx := context.Background()
	if err := initTracingAndMetrics(ctx, traceMode); err != nil {
		log.Fatalf("init failed: %v", err)
	}
	if err := initDB(); err != nil {
		log.Fatalf("DB init failed: %v", err)
	}

	r := gin.Default()
	r.Use(otelgin.Middleware("sqlite-traced-service"))

	r.GET("/user/:id", getUserHandler)

	log.Println("Running on :8080")
	r.Run(":8080")
}

func initTracingAndMetrics(ctx context.Context, mode string) error {
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("sqlite-service"),
		),
	)

	var traceExp sdktrace.SpanExporter
	var err error

	switch mode {
	case "stdout":
		traceExp, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	case "jaeger":
		traceExp, err = jaeger.New(jaeger.WithCollectorEndpoint(
			jaeger.WithEndpoint("http://localhost:14268/api/traces"),
		))
	default:
		log.Fatalf("Unknown trace mode: %s", mode)
	}
	if err != nil {
		return err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Metrics: stdout only (can be extended later)
	metricExp, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
	if err != nil {
		return err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	meter = mp.Meter("sqlite-metrics")
	reqCount, _ = meter.Int64Counter("http_requests_total")
	queryTime, _ = meter.Float64Histogram("db_query_duration_ms")

	return nil
}

func connectDB() *sql.DB {
	db, err := otelsql.Open("sqlite3", "file:test.db?cache=shared&mode=memory", otelsql.WithAttributes(
		semconv.DBSystemSqlite,
	))
	if err != nil {
		log.Fatal(err)
	}

	err = otelsql.RegisterDBStatsMetrics(db, otelsql.WithAttributes(
		semconv.DBSystemSqlite,
	))
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func initDB() error {
	db = connectDB()

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`INSERT INTO users (id, name) VALUES (?, ?)`, 1, "Alice")
	if err != nil {
		log.Println("Insert may have already been done:", err)
	}

	return nil
}

func getUserHandler(c *gin.Context) {
	ctx := c.Request.Context()
	tr := otel.Tracer("handler")
	_, span := tr.Start(ctx, "getUserHandler")
	defer span.End()

	id := c.Param("id")

	reqCount.Add(ctx, 1, metric.WithAttributes(attribute.String("route", "/user/:id")))

	name, err := fetchUserByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id, "name": name})
}

func fetchUserByID(ctx context.Context, id string) (string, error) {
	tr := otel.Tracer("db")
	ctx, span := tr.Start(ctx, "fetchUserByID")
	defer span.End()

	// Simulate additional processing
	time.Sleep(50 * time.Millisecond)

	var name string

	// Trace DB query specifically
	queryCtx, querySpan := tr.Start(ctx, "SQL SELECT")
	start := time.Now()
	err := db.QueryRowContext(queryCtx, "SELECT name FROM users WHERE id = ?", id).Scan(&name)
	queryTime.Record(queryCtx, float64(time.Since(start).Milliseconds()),
		metric.WithAttributes(attribute.String("query", "SELECT user")))
	querySpan.End()

	if err != nil {
		return "", err
	}

	return name, nil
}
