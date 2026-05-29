-- lite-claw initial schema for Supabase (Postgres)
-- Run in Supabase SQL Editor or via supabase db push

create extension if not exists "pgcrypto";

-- Sessions (one per chat / sender)
create table if not exists public.sessions (
  id text primary key,
  channel text not null default 'unknown',
  sender text,
  title text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists sessions_channel_idx on public.sessions (channel);
create index if not exists sessions_updated_at_idx on public.sessions (updated_at desc);

-- Conversation messages (ordered by position)
create table if not exists public.session_messages (
  id bigserial primary key,
  session_id text not null references public.sessions(id) on delete cascade,
  role text not null check (role in ('user', 'assistant', 'tool')),
  content text not null default '',
  tool_call_id text,
  tool_name text,
  position int not null,
  created_at timestamptz not null default now(),
  unique (session_id, position)
);

create index if not exists session_messages_session_idx on public.session_messages (session_id, position);

-- Long-term agent memory (session-scoped or global when session_id is null)
create table if not exists public.memories (
  id bigserial primary key,
  session_id text references public.sessions(id) on delete cascade,
  key text not null,
  value text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index if not exists memories_session_key_idx
  on public.memories (coalesce(session_id, ''), key);

-- Known contacts across channels
create table if not exists public.contacts (
  id bigserial primary key,
  channel text not null,
  external_id text not null,
  display_name text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  unique (channel, external_id)
);

-- Audit log of inbound/outbound messages
create table if not exists public.message_log (
  id bigserial primary key,
  session_id text references public.sessions(id) on delete set null,
  channel text not null,
  direction text not null check (direction in ('in', 'out')),
  sender text,
  content text,
  created_at timestamptz not null default now()
);

create index if not exists message_log_session_idx on public.message_log (session_id, created_at desc);
create index if not exists message_log_channel_idx on public.message_log (channel, created_at desc);

-- Auto-update updated_at on sessions
create or replace function public.set_updated_at()
returns trigger as $$
begin
  new.updated_at = now();
  return new;
end;
$$ language plpgsql;

drop trigger if exists sessions_updated_at on public.sessions;
create trigger sessions_updated_at
  before update on public.sessions
  for each row execute function public.set_updated_at();

drop trigger if exists memories_updated_at on public.memories;
create trigger memories_updated_at
  before update on public.memories
  for each row execute function public.set_updated_at();

-- RLS: enable when using anon key from clients; gateway should use service_role key.
alter table public.sessions enable row level security;
alter table public.session_messages enable row level security;
alter table public.memories enable row level security;
alter table public.contacts enable row level security;
alter table public.message_log enable row level security;

-- Service role bypasses RLS. For anon/authenticated apps, add policies as needed:
-- create policy "service full access" on public.sessions for all using (true);

comment on table public.sessions is 'lite-claw conversation sessions';
comment on table public.session_messages is 'lite-claw chat history per session';
comment on table public.memories is 'lite-claw long-term agent memory';
comment on table public.contacts is 'lite-claw channel contacts';
comment on table public.message_log is 'lite-claw inbound/outbound message audit';
