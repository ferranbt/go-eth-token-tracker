package postgresql

import (
	"testing"

	"github.com/ferranbt/go-eth-token-tracker/store"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var truncateExec = `
DO $$ DECLARE
    r RECORD;
BEGIN
	FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = current_schema()) LOOP
		EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
	END LOOP;
END $$;
`

func testPostgreSQL(t *testing.T) (store.Store, func()) {
	db, err := sqlx.Connect("postgres", "user=postgres dbname=postgres sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	// apply schema
	if _, err := db.Exec(ddl.String("./schema/schema.sql")); err != nil {
		t.Fatal(err)
	}

	p := &Store{db}
	close := func() {
		// remove all the public tables
		if _, err := db.Exec(truncateExec); err != nil {
			t.Fatal(err)
		}
	}
	return p, close
}

func TestStore(t *testing.T) {
	store.TestStore(t, testPostgreSQL)
}
