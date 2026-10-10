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

## MongoDB

Use BSON binary data, not the UUID subtype, because UniqID-X is not a UUID. Use generic binary subtype `0` for broad driver interoperability, or a documented custom subtype where tooling supports it. Add a unique index (MongoDB automatically makes `_id` unique):

```javascript
{
  _id: BinData(0, "<16 raw UniqID-X bytes>"),
  created_at: ISODate("2026-01-01T00:00:00Z")
}
```

Keep `created_at` for date queries and audit semantics. Treat the binary ID as an opaque 16-byte value in MongoDB drivers.
