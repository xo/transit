# D37. The speed targets are proposed, and phase 3 confirms them

Status: Decided, amended by D47.

Ken decided on 2026-09-29, answering question 17, that these are the speed
targets, and that phase 3 confirms them against the C baseline:

1. A parse after one key, and the highlight query on the result, take less
   than 2 ms on a statement of 10 KB.
2. A first parse runs at no less than half the speed of the C runtime.
3. A parse after one key allocates a small, fixed number of times, and the
   number does not grow with the size of the statement.

Each benchmark names the machine, the grammar and the input. A target changes
by a decision if the measurement shows that it is wrong.

D73 records the measurements of phase 3. Targets 1 and 2 hold, and target
3 does not.
