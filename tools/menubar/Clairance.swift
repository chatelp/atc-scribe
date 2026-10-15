// Clairance menu bar launcher.
//
// An icon in the menu bar that says whether production runs, starts and stops
// it, and opens the page. Below, every co-atc instance running on this Mac,
// production and test alike, each with what it serves, its transcription, its
// station link, its log; and what was left running without an instance. It does not run production itself: it runs the same
// script a double-click runs (production_command, runs/production.command on the
// owner's station), so the sign-in, the station link and the clean stop stay in
// one place. Stopping sends that script SIGTERM, which its trap turns into a
// clean stop of co-atc, its sidecar and the station link. Nothing starts on its
// own: production runs only when asked (owner's choice, 10/10).
//
// Settings, all optional, in ~/.config/clairance/launcher.json:
//   repo                the Clairance checkout (default: where it was built)
//   production_command  relative to repo (default: runs/production.command)
//   station_status_url  radio-ctl's /radio/etat, for the line "On air"
//   tailscale_url       copied by "Copy remote address"
//   planning_file       a markdown table of who uses the GPU; production adds
//                       its row while it runs and removes it when it stops

import AppKit
import SwiftUI

struct LauncherConfig: Decodable {
    var repo: String?
    var production_command: String?
    var station_status_url: String?
    var tailscale_url: String?
    var planning_file: String?
}

/// Replaced by build.sh with the checkout the app was built from.
let builtRepo = "@REPO@"

let coAtcPattern = "bin/co-atc -config configs/config.toml"
let planningMarker = "(menu bar launcher)"

@discardableResult
func run(_ path: String, _ args: [String]) -> (status: Int32, out: String) {
    let p = Process()
    p.executableURL = URL(fileURLWithPath: path)
    p.arguments = args
    let pipe = Pipe()
    p.standardOutput = pipe
    p.standardError = Pipe()
    do { try p.run() } catch { return (-1, "") }
    let data = pipe.fileHandleForReading.readDataToEndOfFile()
    p.waitUntilExit()
    return (p.terminationStatus, String(decoding: data, as: UTF8.self))
}

func pids(matching pattern: String) -> [Int32] {
    run("/usr/bin/pgrep", ["-f", pattern]).out
        .split(separator: "\n").compactMap { Int32($0.trimmingCharacters(in: .whitespaces)) }
}

// MARK: - Every instance on this Mac

/// A co-atc process, production or test, and what goes with it.
struct Instance: Identifiable, Sendable {
    var id: Int32 { pid }
    let pid: Int32
    let production: Bool
    let name: String            // "Production", or the config's name ("migration-fake")
    let elapsed: String
    let config: String          // as given on its command line
    let cwd: String
    let port: Int?
    let extraPorts: [Int]
    let staticDir: String
    let www: String             // "migration @ cad595f", or "copy" when not a checkout
    let database: String
    let log: String
    let sidecarURL: String
    let sidecarPid: Int32?
    let sidecarKind: String     // "real model", "fake", or the command's name
    let stationLinks: [Helper]
    // Read from the server and the sidecar after the scan.
    var version = ""
    var summary = ""            // vitalsLine, empty when the server did not answer
    var sidecarHealth = ""

    var url: String? { port.map { "http://127.0.0.1:\($0)/" } }

    var title: String {
        var t = name
        if let p = port { t += " · :\(p)" }
        if !elapsed.isEmpty { t += " · \(elapsed)" }
        return t
    }
}

/// A helper process: a sidecar or a station link.
struct Helper: Identifiable, Sendable {
    var id: Int32 { pid }
    let pid: Int32
    let what: String
    let elapsed: String
    let command: String
}

/// "05:46:58" or "1-02:03:04" (ps etime) as "5 h 47", "1 d 2 h", "3 min".
func shortElapsed(_ etime: String) -> String {
    let dayParts = etime.split(separator: "-")
    let days = dayParts.count == 2 ? Int(dayParts[0]) ?? 0 : 0
    let hms = (dayParts.last ?? "").split(separator: ":").compactMap { Int($0) }
    guard !hms.isEmpty else { return etime }
    let h = hms.count == 3 ? hms[0] : 0
    let m = hms.count >= 2 ? hms[hms.count - 2] : 0
    if days > 0 { return "\(days) d \(h) h" }
    if h > 0 { return "\(h) h \(String(format: "%02d", m))" }
    return "\(m) min"
}

/// The few keys of a co-atc TOML config the launcher reads: section -> key -> raw value.
func readConfig(_ path: String) -> [String: [String: String]] {
    guard let text = try? String(contentsOfFile: path, encoding: .utf8) else { return [:] }
    var out: [String: [String: String]] = [:]
    var section = ""
    for raw in text.components(separatedBy: "\n") {
        // Drop a comment, unless the # is inside quotes.
        var line = ""
        var quoted = false
        for ch in raw {
            if ch == "\"" { quoted.toggle() }
            if ch == "#" && !quoted { break }
            line.append(ch)
        }
        line = line.trimmingCharacters(in: .whitespaces)
        if line.hasPrefix("[") && line.hasSuffix("]") && !line.contains("=") {
            section = String(line.dropFirst().dropLast())
            continue
        }
        guard let eq = line.firstIndex(of: "=") else { continue }
        let key = line[..<eq].trimmingCharacters(in: .whitespaces)
        var value = line[line.index(after: eq)...].trimmingCharacters(in: .whitespaces)
        if value.hasPrefix("\""), value.hasSuffix("\""), value.count >= 2 { value = String(value.dropFirst().dropLast()) }
        out[section, default: [:]][key] = value
    }
    return out
}

func resolve(_ path: String, from dir: String) -> String {
    guard !path.isEmpty else { return "" }
    let full = path.hasPrefix("/") ? path : (dir as NSString).appendingPathComponent(path)
    return (full as NSString).standardizingPath
}

func processCommand(_ pid: Int32) -> (etime: String, command: String) {
    let out = run("/bin/ps", ["-o", "etime=,command=", "-p", String(pid)]).out
        .trimmingCharacters(in: .whitespacesAndNewlines)
    guard let space = out.firstIndex(of: " ") else { return (out, "") }
    return (String(out[..<space]), out[space...].trimmingCharacters(in: .whitespaces))
}

/// lsof's name for one of the process's descriptors ("cwd", "1").
func openPath(_ pid: Int32, fd: String) -> String {
    run("/usr/sbin/lsof", ["-a", "-p", String(pid), "-d", fd, "-Fn"]).out
        .split(separator: "\n").first(where: { $0.hasPrefix("n") }).map { String($0.dropFirst()) } ?? ""
}

func listener(onPort port: Int) -> Int32? {
    run("/usr/sbin/lsof", ["-nP", "-t", "-iTCP:\(port)", "-sTCP:LISTEN"]).out
        .split(separator: "\n").compactMap { Int32($0) }.first
}

/// The branch and commit of a www folder, when it is a checkout's own.
func wwwOrigin(_ staticDir: String) -> String {
    let top = run("/usr/bin/git", ["-C", staticDir, "rev-parse", "--show-toplevel"]).out
        .trimmingCharacters(in: .whitespacesAndNewlines)
    guard !top.isEmpty, resolve("www", from: top) == staticDir else { return "copy" }
    let branch = run("/usr/bin/git", ["-C", top, "rev-parse", "--abbrev-ref", "HEAD"]).out
        .trimmingCharacters(in: .whitespacesAndNewlines)
    let commit = run("/usr/bin/git", ["-C", top, "rev-parse", "--short", "HEAD"]).out
        .trimmingCharacters(in: .whitespacesAndNewlines)
    return "\(branch) @ \(commit)"
}

func sidecarKind(_ command: String) -> String {
    if command.contains("whisper_server.py") { return "real model" }
    if command.contains("fake-sidecar") { return "fake" }
    return command.split(separator: " ").first.map { ($0 as NSString).lastPathComponent } ?? "?"
}

/// Every co-atc process, and the sidecars and station links left without one.
/// Runs off the main thread: ps, lsof and git, a few tens of milliseconds each.
func scanInstances() -> (instances: [Instance], orphans: [Helper]) {
    let coAtc = run("/usr/bin/pgrep", ["-x", "co-atc"]).out
        .split(separator: "\n").compactMap { Int32($0) }
    let links = run("/usr/bin/pgrep", ["-f", "radio-ctl-sync"]).out
        .split(separator: "\n").compactMap { Int32($0) }
        .map { pid -> Helper in
            let c = processCommand(pid)
            return Helper(pid: pid, what: c.command.contains(" -once") ? "station link, once (should run continuously)" : "station link",
                          elapsed: shortElapsed(c.etime), command: c.command)
        }
    let sidecars = (run("/usr/bin/pgrep", ["-f", "whisper_server.py|fake-sidecar"]).out
        .split(separator: "\n").compactMap { Int32($0) })
        .map { pid -> Helper in
            let c = processCommand(pid)
            return Helper(pid: pid, what: "sidecar, \(sidecarKind(c.command))", elapsed: shortElapsed(c.etime), command: c.command)
        }
    var claimed = Set<Int32>()
    var instances: [Instance] = []
    for pid in coAtc {
        let c = processCommand(pid)
        let args = c.command.split(separator: " ").map(String.init)
        let cfgArg = args.firstIndex(of: "-config").flatMap { $0 + 1 < args.count ? args[$0 + 1] : nil } ?? ""
        let cwd = openPath(pid, fd: "cwd")
        let cfgPath = resolve(cfgArg, from: cwd)
        let cfg = readConfig(cfgPath)
        let server = cfg["server"] ?? [:]
        let port = server["port"].flatMap { Int($0) }
        let extra = (server["additional_ports"] ?? "").split(whereSeparator: { !$0.isNumber }).compactMap { Int($0) }
        let staticDir = resolve(server["static_dir"] ?? "www", from: cwd)
        let local = cfg["transcription.local"] ?? [:]
        let sidecarURL = local["server_url"] ?? ""
        let sidecarPort = URL(string: sidecarURL)?.port
        let sidecarPid = sidecarPort.flatMap { listener(onPort: $0) }
        if let s = sidecarPid { claimed.insert(s) }
        let mine = links.filter { h in
            port.map { p in h.command.contains("127.0.0.1:\(p)") || h.command.contains("localhost:\(p)") } ?? false
        }
        mine.forEach { claimed.insert($0.pid) }
        let production = c.command.contains(coAtcPattern)
        var name = "Production"
        if !production {
            name = ((cfgPath as NSString).lastPathComponent as NSString).deletingPathExtension
            if name.hasPrefix("config-") { name = String(name.dropFirst("config-".count)) }
            name = "Test · " + name
        }
        instances.append(Instance(
            pid: pid, production: production, name: name, elapsed: shortElapsed(c.etime),
            config: cfgArg, cwd: cwd, port: port, extraPorts: extra,
            staticDir: staticDir, www: wwwOrigin(staticDir),
            database: resolve(cfg["storage"]?["sqlite_base_path"] ?? "", from: cwd),
            log: openPath(pid, fd: "1"),
            sidecarURL: sidecarURL, sidecarPid: sidecarPid,
            sidecarKind: sidecarPid.map { sidecarKind(processCommand($0).command) } ?? "",
            stationLinks: mine))
    }
    instances.sort { ($0.production ? 0 : 1, $0.port ?? 0) < ($1.production ? 0 : 1, $1.port ?? 0) }
    let orphans = (sidecars + links).filter { !claimed.contains($0.pid) }
    return (instances, orphans)
}

func getJSON(_ s: String, timeout: Double = 3) async -> [String: Any]? {
    guard let url = URL(string: s) else { return nil }
    var req = URLRequest(url: url)
    req.timeoutInterval = timeout
    guard let (data, _) = try? await URLSession.shared.data(for: req) else { return nil }
    return try? JSONSerialization.jsonObject(with: data) as? [String: Any]
}

/// "103 aircraft · 5 437 transmissions today · 44 % matched"
func vitalsLine(_ v: [String: Any]) -> String {
    func n(_ k: String) -> Int? { (v[k] as? NSNumber)?.intValue }
    let f = NumberFormatter()
    f.numberStyle = .decimal
    f.locale = Locale(identifier: "fr_FR")
    var parts: [String] = []
    if let a = n("aircraft") { parts.append("\(a) aircraft") }
    if let t = n("transmissions_today") { parts.append("\(f.string(from: NSNumber(value: t)) ?? String(t)) transmissions today") }
    if let m = n("matched_share") { parts.append("\(m) % matched") }
    return parts.joined(separator: " · ")
}

@MainActor
final class Production: ObservableObject {
    @Published var running = false
    @Published var elapsed = ""
    @Published var onAir = "—"
    // The station's own mode word ("chaine" = the airband chain on the air).
    @Published var stationMode = ""
    // The third state of the sign (05-identite.md): production runs and the
    // antenna is on the air.
    var listening: Bool { running && stationMode == "chaine" }
    @Published var sidecar = "—"
    @Published var instances: [Instance] = []
    @Published var orphans: [Helper] = []
    @Published var busy = false
    @Published var note = ""

    let cfg: LauncherConfig
    let repo: String
    var command: String { cfg.production_command ?? "runs/production.command" }
    private var timer: Timer?

    init() {
        let path = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent(".config/clairance/launcher.json")
        if let data = try? Data(contentsOf: path),
           let c = try? JSONDecoder().decode(LauncherConfig.self, from: data) {
            cfg = c
        } else {
            cfg = LauncherConfig()
        }
        repo = cfg.repo ?? builtRepo
        timer = Timer.scheduledTimer(withTimeInterval: 5, repeats: true) { [weak self] _ in
            Task { @MainActor in await self?.refresh() }
        }
        Task { await refresh() }
    }

    func refresh() async {
        let co = pids(matching: coAtcPattern)
        running = !co.isEmpty
        let scan = await Task.detached { scanInstances() }.value
        var found = scan.instances
        for i in found.indices {
            if let u = found[i].url, let v = await getJSON(u + "api/v1/vitals") {
                found[i].version = v["version"] as? String ?? ""
                found[i].summary = vitalsLine(v)
            }
            if !found[i].sidecarURL.isEmpty {
                if let h = await getJSON(found[i].sidecarURL + "/health") {
                    let status = h["status"] as? String ?? "?"
                    let degraded = (h["degraded"] as? [String]) ?? []
                    let q = h["queue"] as? [String: Any]
                    let queue = q.map { " · queue \(($0["first"] as? NSNumber)?.intValue ?? 0)/\(($0["second"] as? NSNumber)?.intValue ?? 0)" } ?? ""
                    found[i].sidecarHealth = (degraded.isEmpty ? status : "\(status): \(degraded.joined(separator: ", "))") + queue
                } else {
                    found[i].sidecarHealth = "not answering"
                }
            }
        }
        instances = found
        orphans = scan.orphans
        if let pid = co.first {
            elapsed = run("/bin/ps", ["-o", "etime=", "-p", String(pid)]).out
                .trimmingCharacters(in: .whitespacesAndNewlines)
            sidecar = await sidecarState()
        } else {
            elapsed = ""
            sidecar = "—"
        }
        let station = await stationOnAir()
        onAir = station.title
        stationMode = station.mode
        // A start ends when co-atc runs, a stop when the script is gone.
        if busy && (running ? note == "Starting…" : pids(matching: command).isEmpty) {
            busy = false
            note = ""
        }
    }

    func start() {
        guard !running, !busy else { return }
        let script = (repo as NSString).appendingPathComponent(command)
        guard FileManager.default.isExecutableFile(atPath: script) else {
            note = "Not found: \(script)"
            return
        }
        let log = (repo as NSString).appendingPathComponent("runs/launcher.log")
        FileManager.default.createFile(atPath: log, contents: nil)
        guard let out = FileHandle(forWritingAtPath: log) else { return }
        out.seekToEndOfFile()
        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/bin/sh")
        p.arguments = [script]
        p.currentDirectoryURL = URL(fileURLWithPath: repo)
        p.standardOutput = out
        p.standardError = out
        p.standardInput = FileHandle.nullDevice // no terminal: the password comes from its file
        do {
            try p.run()
            busy = true
            note = "Starting…"
            setPlanningRow(true)
        } catch {
            note = "Could not start: \(error.localizedDescription)"
        }
    }

    func stop() {
        // The script's trap stops co-atc, its sidecar and the station link. A
        // script started by a double-click in Terminal is stopped the same way.
        let scripts = pids(matching: command)
        if scripts.isEmpty {
            note = "No production script to stop"
            return
        }
        for pid in scripts { kill(pid, SIGTERM) }
        busy = true
        note = "Stopping…"
        setPlanningRow(false)
    }

    func open(_ url: String) {
        if let u = URL(string: url) { NSWorkspace.shared.open(u) }
    }

    // A test instance stops by its own PIDs, never by a pattern (on 14/10 a
    // pattern also stopped the night's measurement): co-atc, the station
    // link that feeds it, and its sidecar when co-atc did not start it.
    func stopInstance(_ i: Instance) {
        var targets: [(Int32, String)] = [(i.pid, "co-atc")]
        targets += i.stationLinks.map { ($0.pid, "station link") }
        if let s = i.sidecarPid {
            let parent = Int32(run("/bin/ps", ["-o", "ppid=", "-p", String(s)]).out
                .trimmingCharacters(in: .whitespacesAndNewlines)) ?? 0
            if parent != i.pid { targets.append((s, "sidecar")) }
        }
        let alert = NSAlert()
        alert.messageText = "Stop \(i.name)?"
        alert.informativeText = targets.map { "\($0.1) (\($0.0))" }.joined(separator: ", ")
        alert.addButton(withTitle: "Stop")
        alert.addButton(withTitle: "Cancel")
        NSApplication.shared.activate(ignoringOtherApps: true)
        guard alert.runModal() == .alertFirstButtonReturn else { return }
        for (pid, _) in targets { kill(pid, SIGTERM) }
        note = "Stopped \(i.name)"
        Task { await refresh() }
    }

    func stopHelper(_ h: Helper) {
        let alert = NSAlert()
        alert.messageText = "Stop the \(h.what) (\(h.pid))?"
        alert.informativeText = h.command
        alert.addButton(withTitle: "Stop")
        alert.addButton(withTitle: "Cancel")
        NSApplication.shared.activate(ignoringOtherApps: true)
        guard alert.runModal() == .alertFirstButtonReturn else { return }
        kill(h.pid, SIGTERM)
        Task { await refresh() }
    }

    func openFile(_ path: String) {
        guard !path.isEmpty else { return }
        NSWorkspace.shared.open(URL(fileURLWithPath: path))
    }

    func copy(_ text: String) {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(text, forType: .string)
        note = "Copied \(text)"
    }

    func openLog() {
        NSWorkspace.shared.open(URL(fileURLWithPath: (repo as NSString).appendingPathComponent("runs/production.log")))
    }

    func copyRemoteAddress() {
        guard let url = cfg.tailscale_url else { return }
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(url, forType: .string)
        note = "Copied \(url)"
    }

    private func sidecarState() async -> String {
        guard let url = URL(string: "http://127.0.0.1:8178/health"),
              let (data, _) = try? await URLSession.shared.data(from: url),
              let j = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return "not answering" }
        let status = j["status"] as? String ?? "?"
        let degraded = (j["degraded"] as? [String]) ?? []
        return degraded.isEmpty ? status : "\(status): \(degraded.joined(separator: ", "))"
    }

    private func stationOnAir() async -> (title: String, mode: String) {
        guard let s = cfg.station_status_url, let url = URL(string: s) else { return ("—", "") }
        var req = URLRequest(url: url)
        req.timeoutInterval = 4
        guard let (data, _) = try? await URLSession.shared.data(for: req),
              let j = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let sel = j["selection"] as? [String: Any],
              let title = sel["titre"] as? String else { return ("station not answering", "") }
        return (title, (j["mode"] as? String) ?? "")
    }

    // The GPU is shared with the lab: production says so while it runs, in the
    // row a person or an agent would otherwise have written by hand.
    private func setPlanningRow(_ on: Bool) {
        guard let path = cfg.planning_file,
              var text = try? String(contentsOfFile: path, encoding: .utf8) else { return }
        var lines = text.components(separatedBy: "\n").filter { !$0.contains(planningMarker) }
        if on, let sep = lines.firstIndex(where: { $0.hasPrefix("|---") }) {
            let f = DateFormatter()
            f.locale = Locale(identifier: "fr_FR")
            f.dateFormat = "dd/MM 'à' HH:mm"
            lines.insert("| depuis le \(f.string(from: Date())), durée ouverte \(planningMarker) | Clairance | **co-atc en production** | oui, en continu (trois modèles, ~8 Go) |", at: sep + 1)
        }
        text = lines.joined(separator: "\n")
        try? text.write(toFile: path, atomically: true, encoding: .utf8)
    }
}

// The same actions without the menu, for a terminal or a Shortcut:
//   Clairance --status | --start | --stop
// They go through the very code the menu's buttons run.
@main
enum Entry {
    static func main() {
        let args = Array(CommandLine.arguments.dropFirst())
        if let cmd = args.first, ["--status", "--start", "--stop"].contains(cmd) {
            MainActor.assumeIsolated { cli(cmd) }
            return
        }
        ClairanceLauncher.main()
    }

    @MainActor static func cli(_ cmd: String) {
        let p = Production()
        func settle(_ seconds: Double) { RunLoop.main.run(until: Date().addingTimeInterval(seconds)) }
        func report() {
            print(p.running ? "production running" + (p.elapsed.isEmpty ? "" : " for \(p.elapsed)") : "production stopped")
            if p.running { print("sidecar: \(p.sidecar)") }
            print("on air: \(p.onAir)")
            for i in p.instances {
                print("")
                print("\(i.title)  (pid \(i.pid))")
                for line in instanceLines(i) { print("  \(line)") }
            }
            if !p.orphans.isEmpty {
                print("")
                print("left without an instance:")
                for h in p.orphans { print("  \(h.what) (pid \(h.pid), \(h.elapsed)): \(h.command)") }
            }
            if !p.note.isEmpty { print("note: \(p.note)") }
        }
        settle(2)
        switch cmd {
        case "--start":
            p.start()
            for _ in 0..<120 where !(p.running && p.sidecar == "ok") { settle(2); Task { await p.refresh() } }
        case "--stop":
            p.stop()
            for _ in 0..<60 where p.running { settle(2); Task { await p.refresh() } }
        default:
            break
        }
        Task { await p.refresh() }
        settle(6)
        report()
        exit(p.note.hasPrefix("Not found") || p.note.hasPrefix("Could not") ? 1 : 0)
    }
}

// What an instance's menu says, and the command line's --status.
func instanceLines(_ i: Instance) -> [String] {
    var lines: [String] = []
    lines.append("Page: \(i.www)")
    if !i.version.isEmpty { lines.append("Server: \(i.version)") }
    lines.append(i.summary.isEmpty ? "Server not answering" : i.summary)
    if !i.sidecarURL.isEmpty {
        let kind = i.sidecarKind.isEmpty ? "" : "\(i.sidecarKind), "
        lines.append("Transcription: \(kind)\(i.sidecarHealth)" + (i.sidecarKind == "real model" ? " · uses the GPU" : ""))
    }
    if i.stationLinks.isEmpty {
        lines.append("Station link: none")
    } else {
        for h in i.stationLinks { lines.append("Station link: \(h.what == "station link" ? "running" : h.what), \(h.elapsed)") }
    }
    let ports = ([i.port].compactMap { $0 } + i.extraPorts).map(String.init).joined(separator: ", ")
    if !ports.isEmpty { lines.append("Ports: \(ports)") }
    lines.append("Config: \(i.config)")
    if !i.database.isEmpty { lines.append("Databases: \(i.database)") }
    return lines
}

// menuBarIcon loads the sign's three states from the bundle's Resources
// (build.sh copies them there), 18 pt: stopped and running are template
// images, tinted by macOS for a light or dark bar; listening keeps its own
// colours, on purpose (05-identite.md). Without the bundle (the command-line
// form), the system symbols stand in.
func menuBarIcon(running: Bool, listening: Bool) -> NSImage {
    let name = listening ? "barre-menus-ecoute" : running ? "barre-menus-marche" : "barre-menus-arret"
    if let url = Bundle.main.url(forResource: name, withExtension: "svg"),
       let image = NSImage(contentsOf: url) {
        image.isTemplate = !listening
        image.size = NSSize(width: 18, height: 18)
        return image
    }
    let fallback = running ? "dot.radiowaves.left.and.right" : "antenna.radiowaves.left.and.right.slash"
    return NSImage(systemSymbolName: fallback, accessibilityDescription: "Clairance") ?? NSImage()
}

struct ClairanceLauncher: App {
    @StateObject private var p = Production()

    var body: some Scene {
        MenuBarExtra {
            Text(p.running ? "Production running" + (p.elapsed.isEmpty ? "" : " for \(p.elapsed)") : "Production stopped")
            if p.running { Text("Sidecar: \(p.sidecar)") }
            Text("On air: \(p.onAir)")
            if !p.note.isEmpty { Text(p.note) }
            Divider()
            Button("Start production") { p.start() }.disabled(p.running || p.busy)
            Button("Stop production") { p.stop() }.disabled(!p.running)
            Divider()
            Button("Open Clairance") { p.open("http://127.0.0.1:8000/") }.disabled(!p.running)
            if p.cfg.tailscale_url != nil { Button("Copy remote address") { p.copyRemoteAddress() } }
            Button("Production log") { p.openLog() }
            Divider()
            Text(p.instances.isEmpty ? "No instance running" : "Instances running: \(p.instances.count)")
            ForEach(p.instances) { i in
                Menu(i.title) {
                    ForEach(instanceLines(i), id: \.self) { Text($0) }
                    Divider()
                    if let u = i.url {
                        Button("Open \(u)") { p.open(u) }
                        Button("Copy address") { p.copy(u) }
                    }
                    Button("Open log") { p.openFile(i.log) }.disabled(i.log.isEmpty || !i.log.hasPrefix("/"))
                    Button("Show config") { p.openFile(resolve(i.config, from: i.cwd)) }
                    if !i.production {
                        Divider()
                        Button("Stop this instance…") { p.stopInstance(i) }
                    }
                }
            }
            if !p.orphans.isEmpty {
                Menu("Left without an instance: \(p.orphans.count)") {
                    ForEach(p.orphans) { h in
                        Button("Stop \(h.what) · \(h.pid) · \(h.elapsed)…") { p.stopHelper(h) }
                    }
                }
            }
            Divider()
            Button("Quit launcher (production keeps running)") { NSApplication.shared.terminate(nil) }
        } label: {
            // The brand's two states (docs-fr/design/05-identite.md): the scope with
            // its sweep and plot while production runs, the empty screen when it is
            // stopped. Template images, so macOS tints them for a light or dark bar.
            Image(nsImage: menuBarIcon(running: p.running, listening: p.listening))
        }
        .menuBarExtraStyle(.menu)
    }
}
