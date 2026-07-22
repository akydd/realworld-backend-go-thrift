-- +goose up
create table if not exists users (
	id serial primary key,
	username varchar(45) not null,
	email varchar(45) not null,
	password varchar(100) not null,
	bio text,
	image varchar(100)
);

-- +goose down
drop table if exists users;
