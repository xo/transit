# D67. The generator logs with log/slog

Status: Decided.

Ken decided on 2026-09-29 that the generator takes an optional
`*slog.Logger` of the package `log/slog` in its options. A nil logger logs
nothing. The command `transit` sets a text handler when a flag asks for the
log. `log/slog` is in the standard library, so the root module still requires
nothing (D11).

The port then writes the lines of `debug!` and `info!` of `build_tables/` of
upstream, at the levels `slog.LevelDebug` and `slog.LevelInfo`, and it ports
`report_state_info` and the option `--report-states-for-rule` of the tool.
The log changes no output of the generator.
