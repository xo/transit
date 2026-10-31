# D123. The MySQL grammar tunes its identifier rules

Status: Decided.

The MySQL grammar that xo writes (D106) has 1,179 large parse states of
7,307, and its package adds 2.8 MB to a binary. A keyword that is not
reserved can be a name in MySQL, and the grammar lists such keywords at the
places where a name can go.

Ken decided on 2026-10-31 that the MySQL grammar reworks its identifier
rules, with the same technique as D122 point 4, so that fewer of its states
are large. The trees of its corpus keep their meaning. A change of the form
of a tree is recorded in the commit.
