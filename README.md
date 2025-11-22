Irata
=====

Irata is a poker clock, or a tournament timer.

Irata was inspired by a particular private event held in a arcade game repair
shop.  The idea of the classic Atari Sprint 2 font on an open-frame monitor was
too tempting, so we made our own clock.

This is pre-release
-------------------

Setup requires a number of poorly documented manual steps.  There are also a
lot of rough corners and even the occasional outright bug.

But, it does kind of work.


Getting Started
---------------

This is currently a fairly inconvenient process, and poorly documented.

You'll need a Postgres database.  The default location is postgres:///irata.
If not, you'll need to use environment variables to configure the Postgres
location.  `dbconnect.go` and `Dockerfile` will provide some hints.

Import the data in `schema.sql`.  This is, in theory, the current database schema.
Importing `example.sql` will provide some useful sample data.  This is very likely
to have bugs, as it is the least-tested portion of an under-tested server.

Run ./BUILD to build iratad, the server, and irataadmin, an administration utility.

Rotate the cookie keys.  `irataadmin key rotate` should do it.  Look at the
help for this, as the default interval for validity is six months.

Use `irataadmin` to create an administrator account for yourself:
`irataadmin --admin --email devnull@your.domain --nick your-nickname` and
pick a password.  Look at `connect.go` for configuring the database location from
the environment.

Run the server and connect a web browser to it.  Hopefully it's straightforward.
Maybe log in.

You can create a tournament, a structure, and a set of "footer plugs" that will
appear at the bottom.  You can control the tournament by viewing it by a
logged-in user.  Press F1 (or ?) to access key bindings.

Productionizing
---------------

TBD.

The current "production" installation is a single server hosted on a single
domain, with an nginx reverse proxy out front providing SSL termination.
This is all ad-hoc, so I don't have much real advice here.

Deploying in Google's Cloud Run environment works, but Cloud Run will gratuitously
restart the server, and it really isn't designed for that.  Dropped client connections
will re-sync after a minute, but sometimes that minute matters.  This is actually
a bit stateful, so you probably want a real server.  (Also, it's cheaper.)

The `iratad` daemon is self-contained.  irata code does not use the filesystem
at runtime (although it appears the GCP SDK does, to get SSL certificates).

`Dockerfile` will build an image that can host the server, perhaps in Google's
Cloud Run environment.  This helps with SSL termination, but causes other
problems.  Cloud Run isn't geared towards long-poll servers.  It is also a very
expensive way to run the server.

The server wants to use the pgx library to connect to the database.  It is easy
to adapt this to use the GCP library, but as I am not using it and the GCP SDK
increases the code size from 21MB to 32MB, I have removed it.

Logging and debug vars are handled through the Go stdlib.  A lot of other
things are under-engineered.


Operation
---------

All clients and all servers are assumed to have synchronized clocks (NTP).

Many clients connect to one server, each displaying a synchronized clock.
Having separate clients is intended to alleviate the need of having a splitter
among many monitors.  The clients can be hosted on a headless Raspberri Pi.
(In the future, we may have a 'kiosk' mode to make administrating these
easier.)

Clients connect to the server on port 8888 by default.

Clients are assumed to display in a roughly landscape mode.  The displayed page
is not truly reactive, and won't look good on a portrait-oriented display like
most tablets and phones.  However, it does adapt trivially for most desktop
monitors and TVs.

Implementation
--------------

This is a straightforward CRUD app--with one wrinkle.  JavaScript in the view
page is used to long-poll the server for changes to the data model.  Time
elapsing is not a change, but a clock pause, add/remove players/buyins/addons
*are* changes to the current data model, and will cause updates on the client.

Clients long-poll the server for updates to the model that they're interested
in.

Database objects look a lot like wire objects.  (We get away with this since
our clients are essentially ephemeral and we can just reload them.)

Non-clock pages are very simple CRUD app pages.  No ORM is used, and only a
little JavaScript is used to make the forms work a little better.


There Are Many Poker Clocks; This One Is Mine
---------------------------------------------

This is a trivial web app with a somewhat straightforward Go backend to scaffold it.
I did not use any web frameworks for this, so the frontend is
pretty old-fashioned.  The resultant pages are somewhat less reactive than they
could be (it will render on a phone but doesn't look good in portrait mode).

The reason for the reactionary no-framework setup is that I can do without
them, and I am afraid they will disappear from active maintenance.  By sticking
close to the stdlib API and well-known packages, I hope to minimize deprecation
costs and security upgrades.

I am not generally opposed to better libraries.  For instance, I suspect I'll
probably eventually swap out the logger with something that can optionally
output structured logs.  On the other hand, I will probably never use an ORM.
I don't like them.

My friend Patrick Milligan wrote a clock known as the Oakleaf Tournament Timer.
It was used by a few proper poker rooms in the '00s, including Bay 101 and
Bellagio.  Much of this clock is inspired by Patrick's work, in particular, the
key bindings are very familiar.  (Patrick's clock is no longer maintained and
has some limitations I didn't want to live with.  It is still more attractive
than Irata.) Patrick has also contributed several usability suggestions, some
of which I even implemented.

There are quite a few inside jokes in the current state of the code.


Fonts
-----

This repo includes a couple fonts stolen from Internet sources.  Their licenses
are in the `license` directory.  These fonts are baked into the `iratad` binary
and referenced by the CSS.

These can be changed.  Any font will work, but the clock will dance around the screen
if the font is not a monospace font.

PressStart2P is honestly monospace (not every arcade-style mono font is), and
has a nice low line-height (no padding), and allowed the displayed pages to
have that very 1981 look.  This is used in the default `irata` theme.

I have used a couple Red Hat fonts.  Red Hat Mono is a current favorite font
for code.  It looks OK here but an unslashed zero for nontechnical users would
probably be better.  Red Hat Display looks a little dense to me but I like it.

To Do
-----

* CORS configuration requires the app to be working already.  That's probably
  not going to end well.
* A lot of inconsistency in data model names and variable names.  The whole
  of `movement.js` is pretty bad.
  * Gratuitious use of LLMs has not helped code consistency.  I regret nothing.
* Limited multi-user support.  Can't assign owners to tournaments.  (If all
  tournaments were public it would maybe be OK.)
  * There are both admin and non-admin users, but non-admin users are useless.
  * Tournaments need an owner and only the owner can modify the tournament.
  * Users have email addresses for no good reason.  These should be verified,
    or at least labeled as verified.
* Data models aren't quite right.  All data is visible without being logged in
  anyway.  Permissions are, "I can edit everything", or "I can't edit
  anything."
* We use long polling instead of web sockets for no good reason.  This is
  actually not that big of a problem, but if you load a whole bunch of browser
  tabs up at the same clock, eventually the browser will starve for
  connections.  This looks like a server bug but isn't.
* SSL isn't supported.  My server is fronted with an nginx which does SSL.
  But the server doesn't know how to get IP addresses correctly.
* QUIC would be fun, but doesn't seem relevant while running in Cloud Run.*
* Pagination isn't supported in many places where it should be.
  Since we have only a trivial number of users, this isn't a problem that
  has risen to the top of the stack yet.
* Changing most thing in site config means restarting the server, but this
  could be detected automatically.  (Writes to these objects are cached
  and the write path is instrumented to invalidate the cache, we just need
  more of this.)
* Tournament changes notify all clients, both through an interceptor in the
  database write code, as well as Postgres notifications.  Not all objects
  are so instrumented and only tournament model changes cause client updates.
* We need a kiosk mode so we can remote-control clients that are just loading
  one of our URLs.
* Theme support is limited.  Themes are built-in.
* Sounds should be in the database, I guess.  They are currently built-in.
* There should be more than one pay table, and some pay table should scale to
  at least 500 players.  Pay tables will probably stay built-in for a long
  time.
* Errors (particularly pay table generation errors) aren't reported well.
* This list keeps getting longer.
