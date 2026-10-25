// retention borra los eventos de más de RETENTION_DAYS días (30 por defecto) y limpia
// códigos de vinculación, solicitudes de emisión y solicitudes de consentimiento vencidos. Pensado para un Cron Job de Render.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/repoutil"
)

func main() {
	days := 30
	if v, err := strconv.Atoi(os.Getenv("RETENTION_DAYS")); err == nil && v > 0 {
		days = v
	}
	url, err := repoutil.DatabaseURL()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatal("no se pudo conectar a la base")
	}
	defer db.Close()

	ev, err := db.Exec(ctx, `DELETE FROM events WHERE created_at < now() - make_interval(days => $1)`, days)
	if err != nil {
		log.Fatalf("borrar eventos: %v", err)
	}
	lc, err := db.Exec(ctx, `DELETE FROM link_codes WHERE expires_at < now()`)
	if err != nil {
		log.Fatalf("borrar códigos: %v", err)
	}
	ir, err := db.Exec(ctx, `DELETE FROM issuance_requests WHERE created_at < now() - interval '1 day'`)
	if err != nil {
		log.Fatalf("borrar solicitudes: %v", err)
	}
	cr, err := db.Exec(ctx, `DELETE FROM consent_requests WHERE created_at < now() - make_interval(days => $1)`, days)
	if err != nil {
		log.Fatalf("borrar consentimientos: %v", err)
	}
	fmt.Printf("retención de %d días: %d eventos, %d códigos, %d solicitudes de emisión y %d de consentimiento borrados\n",
		days, ev.RowsAffected(), lc.RowsAffected(), ir.RowsAffected(), cr.RowsAffected())
}
