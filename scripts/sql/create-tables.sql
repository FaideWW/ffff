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
  lastChangeId TEXT NOT NULL,
  recordedAt TIMESTAMP NOT NULL
);

CREATE INDEX if not exists jewels_by_stash ON jewels (stashId);
CREATE INDEX if not exists jewels_by_league_date ON jewels (league,recordedAt);
CREATE INDEX if not exists jewels_by_date ON jewels (recordedAt);

CREATE TABLE if not exists changesets(
  id BIGSERIAL PRIMARY KEY NOT NULL,
  changeId TEXT UNIQUE NOT NULL,
  nextChangeId TEXT UNIQUE NOT NULL,
  stashCount INTEGER NOT NULL,
  processedAt TIMESTAMP NOT NULL,
  timeTaken INTEGER NOT NULL
);

CREATE INDEX if not exists changesets_by_changeid ON changesets (changeId);
CREATE INDEX if not exists changesets_by_date ON changesets (processedAt);

CREATE TABLE if not exists snapshot_sets(
  id BIGSERIAL PRIMARY KEY NOT NULL,
  exchangeRates JSON NOT NULL,
  generatedAt TIMESTAMP NOT NULL
);

CREATE TABLE if not exists snapshots(
  id BIGSERIAL PRIMARY KEY NOT NULL,
  setId BIGINT NOT NULL,
  jewelType TEXT NOT NULL,
  jewelClass TEXT NOT NULL,
  allocatedNode TEXT NOT NULL,
  league TEXT NOT NULL,
  minPrice REAL NOT NULL,
  firstQuartilePrice REAL NOT NULL,
  medianPrice REAL NOT NULL,
  thirdQuartilePrice REAL NOT NULL,
  maxPrice REAL NOT NULL,
  windowPrice REAL NOT NULL,
  stddev REAL NOT NULL,
  numListed INTEGER NOT NULL,
  generatedAt TIMESTAMP NOT NULL,
  CONSTRAINT fk_set FOREIGN KEY(setId) REFERENCES snapshot_sets(id)
);
