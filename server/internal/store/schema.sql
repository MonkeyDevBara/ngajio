CREATE TABLE users(
  id INTEGER PRIMARY KEY,
  sub TEXT NOT NULL UNIQUE,
  email TEXT NOT NULL,
  nama TEXT NOT NULL DEFAULT '',
  plan TEXT NOT NULL DEFAULT 'gratis' CHECK(plan IN('gratis','bayar')),
  rev INTEGER NOT NULL DEFAULT 0,
  nrec INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  last_seen INTEGER NOT NULL);
CREATE TABLE sessions(
  token_hash BLOB PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL, last_seen INTEGER NOT NULL
) WITHOUT ROWID;
CREATE INDEX sessions_user ON sessions(user_id);
CREATE INDEX sessions_exp ON sessions(expires_at);
CREATE TABLE rec(
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  tabel TEXT NOT NULL CHECK(tabel IN('pengajar','metode','kelompok','santri','setoran','target')),
  id TEXT NOT NULL,
  rev INTEGER NOT NULL,
  hapus INTEGER NOT NULL DEFAULT 0 CHECK(hapus IN(0,1)),
  json TEXT NOT NULL,
  PRIMARY KEY(user_id,tabel,id)) WITHOUT ROWID;
CREATE INDEX rec_rev ON rec(user_id,rev);
CREATE TABLE mut(
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  mid TEXT NOT NULL, at INTEGER NOT NULL,
  PRIMARY KEY(user_id,mid)) WITHOUT ROWID;
