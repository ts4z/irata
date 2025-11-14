DROP TABLE structures CASCADE;
DROP TABLE tournaments CASCADE;
DROP TABLE text_footer_plugs CASCADE;
DROP TABLE footer_plug_sets CASCADE;
DROP TABLE users CASCADE;
DROP TABLE passwords CASCADE;
DROP TABLE user_email_addresses CASCADE;
DROP TABLE site_info CASCADE; -- obsolete name
DROP TABLE site_config CASCADE;

CREATE TABLE users (
    user_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nick VARCHAR(20) NOT NULL UNIQUE,
    is_admin BOOLEAN DEFAULT FALSE NOT NULL,
    is_operator BOOLEAN DEFAULT FALSE NOT NULL,
    model_data JSONB DEFAULT '{}' NOT NULL
);

CREATE INDEX idx_user_nick ON users(nick);

CREATE TABLE passwords (
    password_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    hashed_password VARCHAR(255) NOT NULL,
    expires TIMESTAMP WITHOUT TIME ZONE
);

CREATE TABLE user_email_addresses (
    email_address VARCHAR(255) NOT NULL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id)
);

CREATE INDEX idx_user_email_addresses_email 
    ON user_email_addresses(email_address);

CREATE TABLE footer_plug_sets (
   id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
   name TEXT NOT NULL,
   version BIGINT DEFAULT 0 NOT NULL
);

CREATE TABLE text_footer_plugs (
   id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
   version BIGINT DEFAULT 0 NOT NULL,
   footer_plug_set_id BIGINT NOT NULL REFERENCES footer_plug_sets(id) ON DELETE CASCADE,
   text TEXT NOT NuLL
);

-- obsolete
CREATE TABLE site_info (
   key TEXT PRIMARY KEY UNIQUE NOT NULL,
    value JSONB NOT NULL,
    version BIGINT DEFAULT 0 NOT NULL
);

-- Unlike other tables, we will not auto-generate the key here.
-- The keys should be known.
-- key 1 = SiteConfig
CREATE TABLE site_config (
   id BIGINT PRIMARY KEY UNIQUE,
   value JSONB NOT NULL,
   version BIGINT DEFAULT 0 NOT NULL
);
 
CREATE TABLE tournaments (
       tournament_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
       version BIGINT DEFAULT 0 NOT NULL,
       model_data JSONB NOT NULL
);

CREATE INDEX idx_tournaments_handle 
    ON tournaments(handle); 

CREATE TABLE structures (
       structure_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
       version BIGINT DEFAULT 0,
       name TEXT NOT NULL,
       model_data JSONB NOT NULL
);

CREATE OR REPLACE FUNCTION notify_tournaments_change()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('tournaments_changes', json_build_object(
        'Table', 'tournaments',
        'OnID', NEW.tournament_id,
        'Version', NEW.version
    )::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER tournaments_notify
AFTER INSERT OR UPDATE ON tournaments
FOR EACH ROW
EXECUTE FUNCTION notify_tournaments_change();

CREATE OR REPLACE FUNCTION notify_site_config_change()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('site_config_changes', json_build_object(
        'Table', 'site_config',
        'OnID', NEW.key,
        'Version', NEW.version
    )::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER site_config_notify
AFTER INSERT OR UPDATE ON site_config
FOR EACH ROW
EXECUTE FUNCTION notify_site_config_change();
