# Database and API integration

Store `ID` as exactly 16 raw bytes. `BYTEA` is the default because UniqID-X is
not an RFC UUID and the type communicates that semantic clearly. PostgreSQL's
native `uuid` type also preserves the exact 16 bytes and byte ordering, and can
be used when its smaller index footprint is more important than the semantic
distinction. Treat that column as an opaque UniqID-X value and do not expose it
as an RFC UUID.

```sql
-- PostgreSQL
CREATE TABLE orders (
  id BYTEA PRIMARY KEY CHECK (octet_length(id) = 16),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

An opaque native-UUID alternative for PostgreSQL is:

```sql
CREATE TABLE orders (
  id UUID PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

```sql
-- MySQL
CREATE TABLE orders (
  id BINARY(16) NOT NULL PRIMARY KEY,
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
);
```

The application must still bind and validate the original `uniqidx.ID`; the
database type does not validate the UniqID-X version, mode, variant, timestamp,
token, or sequence fields. `ID.Value()` and `ID.Scan()` bind raw binary values
through `database/sql`. JSON uses `MarshalText`, which emits canonical
`UX1V-...` text. Never emit a 128-bit ID as a JSON number; JavaScript numeric
values cannot represent it exactly.

## MongoDB

Use BSON binary data, not the UUID subtype, because UniqID-X is not a UUID. Use generic binary subtype `0` for broad driver interoperability, or a documented custom subtype where tooling supports it. Add a unique index (MongoDB automatically makes `_id` unique):

```javascript
{
  _id: BinData(0, "<16 raw UniqID-X bytes>"),
  created_at: ISODate("2026-01-01T00:00:00Z")
}
```

Keep `created_at` for date queries and audit semantics. Treat the binary ID as an opaque 16-byte value in MongoDB drivers.
