Irata
=====

Irata is a poker clock, that is, a tournament timer.

Irata was inspired by a particular private event held in a arcade game repair
shop.  The idea of the classic Atari Sprint 2 font on an open-frame monitor was
too tempting.

This is a Prototype
-------------------

This isn't complete, and it doesn't work right in many common cases.

As of this (hopefully unpublished) writing, this clock works as something of a
demo.  It will keep time, mostly, but there are glaring bugs around times,
and the notion of client/server is badly abused.

This should write to stable storage to allow a server restart (at least!) and
right now it doesn't, it doesn't even manage locks correctly in memory.

This lacks security entirely.

Installing
----------

`go build .`

Run the server and connect a web browser to it.

There is no way to edit the poker tournament.  To the extent the clock works at
all, it uses a baked-in example tournament (not coincidentally, the one I
intended to run).  You may need to do some work here.


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
well-handled; in particular, JavaScript's time handling is pretty mediocre, and
there is no current facility for integrating proper JS libraries into the app.

My friend Patrick Milligan wrote a clock known as the Oakleaf Tournament Timer.
It was used by a few proper poker rooms in the '00s, notably Bay 101 and
Bellagio.  Some of this clock is inspired by Patrick's work, in particular, the
key bindings are very familiar.  (Patrick's clock is no longer available.)

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

State needs to be stored.

State is one level, and times do not need to be present on all levels, just the
active one.  (This may mean a forward-back needs a special case)
