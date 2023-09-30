CREATE TABLE if not exists jewels(
  id BIGSERIAL PRIMARY KEY NOT NULL,
  jewelType TEXT NOT NULL,
  jewelClass TEXT NOT NULL,
  allocatedNode TEXT NOT NULL,
  stashId TEXT NOT NULL,
  league TEXT NOT NULL,
  itemId TEXT UNIQUE NOT NULL,
  listPriceAmount REAL NOT NULL,
  listPriceCurrency TEXT NOT NULL,
  recordedAt TIMESTAMP NOT NULL
);

CREATE INDEX if not exists jewels_by_stash ON jewels (stashId);
CREATE INDEX if not exists jewels_by_league_date ON jewels (league,recordedAt);
CREATE INDEX if not exists jewels_by_date ON jewels (recordedAt);

