---
name: golang-database
description: >-
  Acceso seguro, robusto e idiomático desde Go a bases de datos relacionales (PostgreSQL, MariaDB, MySQL y SQLite).
  Uso de database/sql, pools de conexiones, consultas preparadas, transacciones atómicas seguras y configuración óptima de SQLite.
---

# Go Database Best Practices Guide

Esta skill detalla los patrones y estándares indispensables para la interacción con bases de datos en Go a través de `database/sql`.

---

## 1. Ciclo de Vida y Connection Pooling

- **`*sql.DB` es un pool concurrente:** Una instancia de `*sql.DB` es segura para uso concurrente desde múltiples goroutines. Debe crearse una sola vez al iniciar la aplicación y cerrarse al finalizar (`defer db.Close()`).
- **Configuración obligatoria del pool:**
  ```go
  db.SetMaxOpenConns(25)
  db.SetMaxIdleConns(25)
  db.SetConnMaxLifetime(5 * time.Minute)
  db.SetConnMaxIdleTime(1 * time.Minute)
  ```

---

## 2. Optimización Específica para SQLite

Cuando se utiliza SQLite como base de datos embebida (común en aplicaciones de escritorio como AppSire):
- **Modo WAL (Write-Ahead Logging):** Imprescindible para permitir lecturas concurrentes sin bloquear escrituras:
  ```go
  db, err := sql.Open("sqlite3", "file:data.db?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
  ```
- **Concurrencia de escritura en SQLite:** Dado que SQLite solo permite un escritor activo simultáneamente, se debe configurar un `_busy_timeout` adecuado (ej. 5000 ms) o limitar `SetMaxOpenConns(1)` en entornos con alta contención.

---

## 3. Manejo Seguro de Transacciones

Siempre utilizar el patrón `defer tx.Rollback()` para garantizar que la transacción se libere en caso de error o pánico:

```go
func Transfer(ctx context.Context, db *sql.DB, fromID, toID int, amount float64) error {
    tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
    if err != nil {
        return fmt.Errorf("iniciar transacción: %w", err)
    }
    // Si Commit tiene éxito, Rollback no tiene efecto y retorna sql.ErrTxDone
    defer tx.Rollback()

    if _, err := tx.ExecContext(ctx, "UPDATE accounts SET balance = balance - ? WHERE id = ?", amount, fromID); err != nil {
        return fmt.Errorf("debitar cuenta: %w", err)
    }
    if _, err := tx.ExecContext(ctx, "UPDATE accounts SET balance = balance + ? WHERE id = ?", amount, toID); err != nil {
        return fmt.Errorf("acreditar cuenta: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("confirmar transacción: %w", err)
    }
    return nil
}
```

---

## 4. Iteración Segura sobre `sql.Rows`

Para prevenir fugas de conexiones en el pool, siempre cerrar `rows` e inspeccionar `rows.Err()`:

```go
rows, err := db.QueryContext(ctx, "SELECT id, ruc, name FROM companies")
if err != nil {
    return nil, fmt.Errorf("consultar empresas: %w", err)
}
defer rows.Close()

var list []Company
for rows.Next() {
    var c Company
    if err := rows.Scan(&c.ID, &c.RUC, &c.Name); err != nil {
        return nil, fmt.Errorf("scan fila: %w", err)
    }
    list = append(list, c)
}

// Comprobar siempre errores durante la iteración
if err := rows.Err(); err != nil {
    return nil, fmt.Errorf("iteración de filas: %w", err)
}
return list, nil
```
