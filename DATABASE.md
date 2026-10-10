# Database and API integration

Store `ID` as exactly 16 raw bytes. Do not use UUID columns: UniqID-X is not an RFC UUID.

```sql
-- PostgreSQL
CREATE TABLE orders (
  id BYTEA PRIMARY KEY CHECK (octet_length(id) = 16),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- MySQL
CREATE TABLE orders (
  id BINARY(16) NOT NULL PRIMARY KEY,
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
);
```

`ID.Value()` and `ID.Scan()` bind raw binary values through `database/sql`. JSON uses `MarshalText`, which emits canonical `UX1V-...` text. Never emit a 128-bit ID as a JSON number; JavaScript numeric values cannot represent it exactly.
