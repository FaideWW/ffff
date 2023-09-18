CREATE TABLE if not exists jewels(
  id BIGSERIAL PRIMARY KEY NOT NULL,
  jewelType CHAR(50) NOT NULL,
  jewelClass CHAR(50) NOT NULL,
  allocatedNode CHAR(50) NOT NULL,
  stashX INTEGER NOT NULL,
  stashY INTEGER NOT NULL,
  itemId CHAR(64) UNIQUE NOT NULL,
  stashId CHAR(64) NOT NULL,
  listPriceChaos REAL,
  listPriceDivines REAL,
  recordedAt TIMESTAMP NOT NULL
);

CREATE INDEX if not exists jewels_by_stash ON jewels (stashId);
CREATE INDEX if not exists jewels_by_date ON jewels (recordedAt);
