# Menu bar launcher (macOS)

An icon in the menu bar that says whether co-atc's production runs, starts and
stops it, and opens the page. Nothing starts on its own: production runs only
when asked.

It does not run production itself. It runs the script a double-click would
(`production_command`, by default `runs/production.command`), so sign-in, the
station link and the clean stop stay in one place, and stops it with SIGTERM,
which that script's trap turns into a clean stop of co-atc, its sidecar and the
station link. A script started from Terminal is stopped the same way.

```sh
tools/menubar/build.sh --install   # builds atc-scribe.app, copies it to ~/Applications
```

Needs Xcode's command line tools and macOS 13 or later. Optional settings in
`~/.config/atc-scribe/launcher.json` (outside the repository):

```json
{
  "repo": "/path/to/atc-scribe",
  "production_command": "runs/production.command",
  "station_status_url": "http://your-station/radio/etat",
  "tailscale_url": "http://100.x.y.z:8000/",
  "planning_file": "/path/to/a/markdown/table/of/gpu/use.md"
}
```

The same actions from a terminal or a Shortcut, through the same code:

```sh
~/Applications/atc-scribe.app/Contents/MacOS/atc-scribe --status   # or --start, --stop
```
