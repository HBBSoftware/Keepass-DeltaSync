-- Generér en frisk admin-token via SQL (DBeaver-friendly).
--
-- Brug: paste hele scriptet i DBeaver's SQL-editor og kør. Klartekst-tokenet
-- vises som SELECT-resultat ÉN gang — gem det med det samme, det kan ikke
-- hentes igen (kun SHA-256-hashen lagres).
--
-- Token-format matcher TokenHasher.php: 32 random bytes → base64url uden
-- padding. Hash matcher: hex-encoded SHA-256 over klartekst-tokenet.
--
-- Kræver pgcrypto-extension (aktiveret i migration 001).
--
-- SPDX-License-Identifier: AGPL-3.0-or-later

WITH new_token AS (
    -- 32 random bytes → base64 → URL-safe (- _) → uden = padding.
    SELECT rtrim(
               translate(
                   encode(gen_random_bytes(32), 'base64'),
                   '+/',
                   '-_'
               ),
               '='
           ) AS token
),
inserted AS (
    INSERT INTO admin_tokens (token_hash)
    SELECT encode(digest(token, 'sha256'), 'hex')
      FROM new_token
    RETURNING token_hash, created_at
)
SELECT
    nt.token        AS "admin_token",
    i.token_hash    AS "stored_hash",
    i.created_at    AS "created_at"
  FROM new_token nt,
       inserted  i;
