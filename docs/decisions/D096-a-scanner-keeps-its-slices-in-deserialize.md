# D96. A scanner keeps its slices in Deserialize

Status: Decided, amends D85.

Where the `deserialize` of a C scanner frees an array and allocates it again,
the Go port keeps its slice and sets its length to 0. The calls and the state
of the scanner are the same, and a parse after one key allocates less. Ken
decided on 2026-10-01 that this is the rule for every scanner port. The
python scanner does it.
