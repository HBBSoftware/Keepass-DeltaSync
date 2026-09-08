-- KeePass Delta-Sync · Migration 009
-- Rigtigt admin-login: brugernavn + adgangskode i stedet for at kopiere et
-- bearer-token ind i panelet ved hvert besøg.
--
-- Bearer-tokens forsvinder IKKE. admin_tokens bliver stående og virker
-- uændret — bin/admin og klientens `admin`-kommandoer autentificerer stadig
-- med token. Login er en anden vej ind for mennesker med en browser; cookien
-- er HttpOnly, så JavaScript aldrig kan læse den.
--
-- Præcis én administrator: admin_account har en singleton-nøgle med en CHECK,
-- så databasen selv afviser en anden række. Skal der senere være flere,
-- droppes constraint'en og der tilføjes en id-kolonne — sessions peger på
-- token_hash, ikke på en bruger, så den ændring bliver additiv.
-- SPDX-License-Identifier: AGPL-3.0-or-later

CREATE TABLE admin_account (
    singleton     BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    username      TEXT        NOT NULL,
    -- Argon2id fra PHP's password_hash(). Modsat tokens er en adgangskode
    -- gætbar, så her er en langsom, saltet hash det rigtige valg — se
    -- src/Crypto/TokenHasher.php for hvorfor tokens omvendt bruger SHA-256.
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login    TIMESTAMPTZ
);

-- Session-tokens hashes som alle andre tokens: deterministisk SHA-256, så
-- opslaget er O(1) og et databasedump ikke giver levende sessioner.
CREATE TABLE admin_sessions (
    token_hash TEXT        PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_used  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Bruges både af udløbs-filteret ved hvert opslag og af oprydningen.
CREATE INDEX admin_sessions_expires_idx ON admin_sessions (expires_at);

-- Rate limiting af login. Et 256-bit token kan ikke brute-forces, det kan en
-- adgangskode — derfor er tælleren først nødvendig nu. Én række pr. (ip,
-- minut); ældre rækker ryddes op ved skrivning.
CREATE TABLE auth_attempts (
    ip           TEXT        NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    attempts     INTEGER     NOT NULL DEFAULT 0,
    PRIMARY KEY (ip, window_start)
);

CREATE INDEX auth_attempts_window_idx ON auth_attempts (window_start);
