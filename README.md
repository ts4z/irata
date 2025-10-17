Irata
=====

Irata is a poker clock, that is, a tournament timer.

Irata was inspired by a particular private event held in a arcade game repair
shop.  The idea of the classic Atari Sprint 2 font on an open-frame monitor was
too tempting.

This is a Prototype
-------------------

This is limping along, and will keep time.  But it is very rudimentary in a lot
of ways.

See "To Do" below.

Setup
-----

You need a database, the database has to be called "irata", it has to have
schema.sql loaded into it (which also has example data), and it has to be
running on the local host as the same user as the binary, and in the same
filesystem.

This will be fixed, eventually.

Installing
----------

`go build .`

Run the server and connect a web browser to it.

There is no way to edit the poker tournament.  To the extent the clock works at
all, it uses a baked-in example tournament (not coincidentally, the one I
intended to run).  You may need to do some work here.

In `cmd/irataadmin`, there is a command-line utility which will allow you to
create a user. 


Operation
---------

The intent is for many clients to connect to one server.  Having separate
clients is intended to alleviate the need of having a splitter among many
monitors.  The clients could be a Raspberry Pi.

Clients connect to the server on port 8888 (for now) and will need the whole
URL, as tournament selection is not implemented.

Clients are expected to display on a 4x3 or 16x9 monitor in landscape mode.
The displayed page is not truly reactive, and won't look good on a
portrait-oriented display like most tablets and phones.  However, it does adapt
trivially for most desktop monitors and TVs.

Setting this all up is left as an exercise to the reader.


There Are Many Poker Clocks; This One Is Mine
---------------------------------------------

This is a trivial web app with a trivial Go backend to scaffold it.  I was too
lazy to learn a web framework for something this trivial, so the frontend is
pretty old-fashioned.  As a result, a lot of the corner cases are not
well-handled.

My friend Patrick Milligan wrote a clock known as the Oakleaf Tournament Timer.
It was used by a few proper poker rooms in the '00s, notably Bay 101 and
Bellagio.  Much of this clock is inspired by Patrick's work, in particular, the
key bindings are very familiar.  (Patrick's clock is no longer sold, and
requires Windows.)

There are quite a few inside jokes in the current state of the code as well.


Fonts
-----

This repo embeds and includes a font called Press Start 2P by CodeMan38.
When built, the `irata` binary will include a copy of this font.  It is covered
by its own license.

https://fonts.google.com/specimen/Press+Start+2P/about

If this is a problem, any font will work, but the current style is intended for
a monospace font.  PressStart2P is honestly monospace, and has a nice low
line-height (no padding), and allowed the displayed pages to have that very
1981 look.


To Do
-----

* A lot of inconsistency in data model names and variable names.  The whole
  of `movement.js` is pretty bad.
* No multi-user support.  Can't assign owners to tournaments.
  (If all tournaments were public it would maybe be OK.)
* Data models aren't quite right.
* Tournaments need an owner and only the owner can modify the tournament.
* There are both admin and non-admin users, but non-admin users are useless.
  All data is visible without being logged in anyway.  Permissions are, "I can
  edit everything", or "I can't edit anything."
* Users have email addresses for no good reason.
* We use long polling instead of web sockets for no good reason.  This is
  actually not that big of a problem, but if you load a whole bunch of browser
  tabs up at the same clock, eventually the browser will starve for
  connections.  This looks like a server bug but isn't.
* SSL isn't supported.  Since I am running this in a Cloud Run instance,
  this is not currently a problem.
* Pagination isn't supported.  Since we have only a trivial number of users,
  this isn't a problem.
* irata.go is way too long.
* Changing anything in site config means restarting the server, but this
  could be detected automatically.
* Database doesn't notify for changes, so we can really only have a single
  server instance if we want things to work reliably.
