Irata
=====

Irata is a poker clock, or a tournament timer.

Irata was inspired by a particular private event held in a arcade game repair
shop.  The idea of the classic Atari Sprint 2 font on an open-frame monitor was
too tempting.

This is pre-release
-------------------

Setup requires a number of poorly documented manual steps.  There are also a
lot of rough corners and even the occasional outright bug.

But, it does kind of work.


Getting Started
---------------

This is currently a fairly inconvenient process.

You'll need a Postgres database.  If it is accessible via postgres:///irata,
that will help.  If not, you'll need to use environment variables to configure
the Postgres location.  `Dockerfile` will provide some hints.

Import the data in `schema.sql`.  This is, in theory, the current database schema.
Importing `example.sql` will provide some useful sample data.

`go build cmd/iratad/iratad.go` to build the server.

`go build cmd/irataadmin/irataadmin.go` to build the command-line utility.
(This needs direct access to the database.)

Use `irataadmin` to create an administrator account for yourself:
`irataadmin --admin --email devnull@your.domain --nick your-nickname` and
pick a password.

Run the server and connect a web browser to it.  Hopefully it's straightforward.
Maybe log in.

You can create a tournament, a structure, and a set of "footer plugs" that will
appear at the bottom.  You can control the tournament by viewing it by a
logged-in user.  Press F1 (or ?) to access key bindings.

Productionizing
---------------

TBD.

`Dockerfile` will build an image that can host the server, perhaps in Google's
Cloud Run environment.  (This helps work around the lack of SSL and logging
support, but Cloud Run is not particularly amenable to the long-polls the JS
client uses to connect to the server.  It seems to work at low-scale, but the
graphs are hysterically terrible.)


Operation
---------

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

This is a pretty straightforward CRUD app, with one wrinkle.  JavaScript in the
view page is used to long-poll the server for changes to the data model.  Time
elapsing is not a change, but a clock pause, add/remove players/buyins/addons
*are* changes to the current data model, and will cause updates on the client.

Clients long-poll the server for updates to the model that they're interested
in.

Non-clock pages are very simple CRUD app pages.  No ORM is used, and only a
little JavaScript is used to make the forms work a little better.


There Are Many Poker Clocks; This One Is Mine
---------------------------------------------

This is a trivial web app with a somewhat straightforward Go backend to scaffold it.
I did not use any web frameworks for this, so the frontend is
pretty old-fashioned.  The resultant pages are somewhat less reactive than they
could be (it will render on a phone but doesn't look good in portrait mode).

My friend Patrick Milligan wrote a clock known as the Oakleaf Tournament Timer.
It was used by a few proper poker rooms in the '00s, notably Bay 101 and
Bellagio.  Much of this clock is inspired by Patrick's work, in particular, the
key bindings are very familiar.  (Patrick's clock is no longer maintained and has
some limitations I didn't want to live with.)  Patrick has also contributed
several usability suggestions, some of which I even implemented.

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
* SSL isn't supported.  Since I am running this in a Cloud Run instance,
  this is not currently a problem.  Also, Let's Encrypt should be supported.
* Pagination isn't supported in many places where it should be.
  Since we have only a trivial number of users, this isn't a problem that
  has risen to the top of the stack yet.
* Changing anything in site config means restarting the server, but this
  could be detected automatically.
* Database doesn't notify for changes, so we can really only have a single
  server instance if we want things to work reliably.  (The database code
  has proper locks against write-write conflicts.)
* We need a kiosk mode so we can remote-control clients that are just loading
  one of our URLs.
* Theme support is *very* poor, and limited to selecting a CSS file.
  Only the "irata" theme is allegedly well tested.
* Sounds should be in the database, I guess.
* There should be more than one pay table, and some pay table should scale to 
  at least 500 players.
* Errors (particularly pay table errors) aren't reported well.
* This list keeps getting longer.
