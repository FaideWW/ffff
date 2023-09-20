CREATE TABLE if not exists jewels(
  id BIGSERIAL PRIMARY KEY NOT NULL,
  jewelType CHAR(50) NOT NULL,
  jewelClass CHAR(50) NOT NULL,
  allocatedNode CHAR(50) NOT NULL,
  stashId CHAR(64) NOT NULL,
  league CHAR(64) NOT NULL,
  itemId CHAR(64) UNIQUE NOT NULL,
  listPriceChaos REAL NOT NULL,
  listPriceDivines REAL NOT NULL,
  recordedAt TIMESTAMP NOT NULL
);

CREATE INDEX if not exists jewels_by_stash ON jewels (stashId);
CREATE INDEX if not exists jewels_by_date ON jewels (recordedAt);

