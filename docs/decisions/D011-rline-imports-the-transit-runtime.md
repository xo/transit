# D11. rline imports the transit runtime

Status: Decided.

Ken decided on 2026-09-29 that rline gets parse information by importing the
transit runtime directly. rline does not use an adapter in usql, and transit
does not publish a separate module of shared types.

Two rules follow from this:

1. The root module of transit, which holds the runtime, imports only the Go
   standard library. A program that imports rline then gets no third party
   dependency from transit. D15 holds the rule for dependencies.
2. rline imports no grammar. Its caller, such as usql, gives it a
   `*transit.Language`, and rline highlights any language with it.

rline has a rule of its own that lists its dependencies. A change to that rule
is a decision of rline, and not of transit.
