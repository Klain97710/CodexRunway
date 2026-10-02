import CFNetwork
import Foundation

/// Opt-in local diagnostics. Never retain URLs, headers, response bodies, or error descriptions.
enum RunwayNetworkDiagnostics {
    enum Route: String, Sendable {
        case system, http, socks5, injected

        init(_ mode: NetworkProxyMode) {
            switch mode {
            case .system: self = .system
            case .http: self = .http
            case .socks5: self = .socks5
            }
        }
    }

    struct Event: Sendable {
        let operation: String
        let route: Route
        let attempt: Int
        let elapsedMilliseconds: Int
        let status: Int?
        let errorDomain: String?
        let errorCode: Int?

        init(request: URLRequest, route: Route, attempt: Int, elapsed: TimeInterval,
             response: URLResponse? = nil, error: Error? = nil)
        {
            operation = Self.operation(request.url)
            self.route = route
            self.attempt = attempt
            elapsedMilliseconds = Int(max(0, elapsed) * 1_000)
            status = (response as? HTTPURLResponse)?.statusCode
            if let error {
                let native = error as NSError
                if [NSURLErrorDomain, NSPOSIXErrorDomain, kCFErrorDomainCFNetwork as String].contains(native.domain) {
                    errorDomain = native.domain
                    errorCode = native.code
                } else {
                    errorDomain = error is CancellationError ? "cancelled"
                        : error is NetworkProxyError ? "proxy" : "other"
                    errorCode = nil
                }
            } else {
                errorDomain = nil
                errorCode = nil
            }
        }

        var line: String {
            "RunwayNetwork operation=\(operation) route=\(route.rawValue) attempt=\(attempt) "
                + "elapsed_ms=\(elapsedMilliseconds) status=\(status.map(String.init) ?? "none") "
                + "error=\(errorDomain ?? "none") code=\(errorCode.map(String.init) ?? "none")"
        }

        private static func operation(_ url: URL?) -> String {
            guard let url else { return "other" }
            if url.host == "chatgpt.com" {
                switch url.path {
                case "/backend-api/wham/usage": return "quota"
                case "/backend-api/wham/rate-limit-reset-credits": return "resetCredits"
                case "/backend-api/wham/profiles/me": return "profileUsage"
                case "/backend-api/wham/analytics/daily-workspace-usage-counts": return "dailyUsage"
                case "/backend-api/accounts": return "workspaceName"
                default: return "other"
                }
            }
            if url.host == "didcodexreset.com", url.path == "/api/status.json" { return "resetToday" }
            return "other"
        }
    }

    @TaskLocal static var observer: (@Sendable (Event) -> Void)?
    private static let enabled = ProcessInfo.processInfo.environment["CODEX_RUNWAY_NETWORK_DIAGNOSTICS"] == "1"

    static func measure(
        request: URLRequest,
        route: Route,
        attempt: Int = 1,
        operation: () async throws -> (Data, URLResponse)) async throws -> (Data, URLResponse)
    {
        guard enabled || observer != nil else { return try await operation() }
        let started = ProcessInfo.processInfo.systemUptime
        do {
            let result = try await operation()
            emit(Event(request: request, route: route, attempt: attempt,
                       elapsed: ProcessInfo.processInfo.systemUptime - started, response: result.1))
            return result
        } catch {
            emit(Event(request: request, route: route, attempt: attempt,
                       elapsed: ProcessInfo.processInfo.systemUptime - started, error: error))
            throw error
        }
    }

    private static func emit(_ event: Event) {
        if let observer { observer(event) }
        if enabled { FileHandle.standardError.write(Data((event.line + "\n").utf8)) }
    }
}
