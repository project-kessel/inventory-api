package jobs

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"context"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// postgresTestImage is the Postgres image used for the real-Postgres retention
	// job tests. The jobs' SQL is Postgres-only ((? || ' days')::INTERVAL, NOW()),
	// so these behavioral tests must run the actual production SQL.
	postgresTestImage = "postgres:17-alpine"
	// postgresTestPassword is the password of the throwaway container's superuser.
	postgresTestPassword = "test"
)

// postgresTestConn holds the connection parameters of the Postgres instance the
// retention tests run against — either a testcontainer started by TestMain or an
// externally provided DSN (KESSEL_POSTGRES_TEST_DSN, e.g. the make db/setup
// container on port 5435, which sidesteps podman-socket friction).
type postgresTestConn struct {
	host     string
	port     string
	user     string
	password string
	adminDB  string
	sslmode  string
}

var (
	postgresTestInstance   *postgresTestConn
	postgresTestSkipReason = "skipping: short mode (run `make test-retention-pg` for the Postgres suite)"
	// cleanupContainer terminates the testcontainer after m.Run(). os.Exit
	// skips defers, so a plain `defer cleanup()` before os.Exit would leak
	// the container on podman, where the Ryuk reaper usually cannot run.
	cleanupContainer func()
)

// TestMain starts one Postgres instance for the whole package when not in short
// mode. `make test` runs -short, so the container never starts there and every
// Postgres test skips cleanly — daemon-less `make test` stays green. If startup
// fails (no container daemon reachable) the tests also skip, with the reason.
func TestMain(m *testing.M) {
	flag.Parse()

	if !testing.Short() {
		if dsn := os.Getenv("KESSEL_POSTGRES_TEST_DSN"); dsn != "" {
			conn, err := parsePostgresTestDSN(dsn)
			if err != nil {
				postgresTestSkipReason = fmt.Sprintf("skipping: invalid KESSEL_POSTGRES_TEST_DSN: %v", err)
			} else {
				postgresTestInstance = conn
			}
		} else {
			conn, cleanup, err := startPostgresTestContainer()
			if err != nil {
				postgresTestSkipReason = fmt.Sprintf("skipping: could not start postgres test container: %v", err)
			} else {
				postgresTestInstance = conn
				cleanupContainer = cleanup
			}
		}
	}

	code := m.Run()
	if cleanupContainer != nil {
		cleanupContainer()
	}
	os.Exit(code)
}

// requirePostgres skips the test when no Postgres instance is available (short
// mode, missing/unusable DSN, or no container daemon).
func requirePostgres(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip(postgresTestSkipReason)
	}
	if postgresTestInstance == nil {
		t.Skip(postgresTestSkipReason)
	}
}

// startPostgresTestContainer starts the throwaway Postgres container. The wait
// strategy needs the SECOND "ready to accept connections" log line: the init
// step restarts the server once, so waiting for the first occurrence (or just
// the listening port) is racy.
func startPostgresTestContainer() (*postgresTestConn, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ctr, err := testcontainers.Run(ctx, postgresTestImage,
		testcontainers.WithEnv(map[string]string{"POSTGRES_PASSWORD": postgresTestPassword}),
		testcontainers.WithExposedPorts("5432/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(3*time.Minute),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not start postgres container: %w", err)
	}

	port, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, nil, fmt.Errorf("could not get postgres port: %w", err)
	}

	conn := &postgresTestConn{
		host:     "localhost",
		port:     port.Port(),
		user:     "postgres",
		password: postgresTestPassword,
		adminDB:  "postgres",
		sslmode:  "disable",
	}
	cleanup := func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			fmt.Printf("could not terminate postgres test container, please remove manually: %v\n", err)
		}
	}
	return conn, cleanup, nil
}

// parsePostgresTestDSN accepts both URL (postgres://user:pass@host:port/db?...)
// and keyword ("host=... user=...") DSN forms and extracts the connection
// parameters. The database named in the DSN is used as the admin database from
// which per-test databases are created.
func parsePostgresTestDSN(dsn string) (*postgresTestConn, error) {
	conn := &postgresTestConn{sslmode: "disable"}

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return nil, fmt.Errorf("parsing DSN URL: %w", err)
		}
		if u.Hostname() == "" {
			return nil, fmt.Errorf("DSN URL has no host")
		}
		conn.host = u.Hostname()
		if p := u.Port(); p != "" {
			conn.port = p
		} else {
			conn.port = "5432"
		}
		if u.User != nil {
			conn.user = u.User.Username()
			if pw, ok := u.User.Password(); ok {
				conn.password = pw
			}
		}
		if u.Path != "" && u.Path != "/" {
			conn.adminDB = strings.TrimPrefix(u.Path, "/")
		} else {
			conn.adminDB = "postgres"
		}
		if ssl := u.Query().Get("sslmode"); ssl != "" {
			conn.sslmode = ssl
		}
	} else {
		// keyword=value form, whitespace separated (values may be single-quoted)
		for kv := range strings.FieldsSeq(dsn) {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("malformed DSN segment %q", kv)
			}
			value := strings.Trim(parts[1], "'")
			switch parts[0] {
			case "host":
				conn.host = value
			case "port":
				conn.port = value
			case "user":
				conn.user = value
			case "password":
				conn.password = value
			case "dbname":
				conn.adminDB = value
			case "sslmode":
				conn.sslmode = value
			}
		}
	}

	if conn.host == "" || conn.user == "" || conn.adminDB == "" {
		return nil, fmt.Errorf("DSN must provide host, user and dbname")
	}
	return conn, nil
}

// dsn returns a keyword DSN for the named database on this instance.
func (c *postgresTestConn) dsn(dbName string) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.host, c.port, c.user, c.password, dbName, c.sslmode)
}
