import Foundation
import Testing
@testable import CodexRunwayCore

@Suite("Network diagnostics")
struct RunwayNetworkDiagnosticsTests {
    @Test("diagnostics contain only allowlisted fields, even for a malicious URL or error")
    func redactsSecrets() throws {
        var request = URLRequest(url: try #require(URL(string:
            "https://fixture-user:fixture-password@chatgpt.com/backend-api/wham/usage?account=fixture-account&token=fixture-secret")))
        request.setValue("Bearer fixture-secret", forHTTPHeaderField: "Authorization")
        request.setValue("fixture-account", forHTTPHeaderField: "ChatGPT-Account-Id")
        request.httpBody = Data("fixture-body".utf8)
        let error = NSError(domain: "fixture-secret", code: 123, userInfo: [
            NSLocalizedDescriptionKey: "fixture-description",
            NSURLErrorFailingURLErrorKey: request.url!,
        ])
        let event = RunwayNetworkDiagnostics.Event(
            request: request, route: .http, attempt: 2, elapsed: 1.25, error: error)
        #expect(event.line == "RunwayNetwork operation=quota route=http attempt=2 elapsed_ms=1250 status=none error=other code=none")
        #expect(!event.line.contains("fixture"))
        request.url = URL(string: "https://chatgpt.com/private/fixture-secret")!
        let unknown = RunwayNetworkDiagnostics.Event(request: request, route: .system, attempt: 1, elapsed: 0)
        #expect(unknown.operation == "other")
        request.url = URL(string: "https://fixture-secret.example/backend-api/wham/usage")!
        #expect(RunwayNetworkDiagnostics.Event(request: request, route: .system, attempt: 1, elapsed: 0).operation == "other")
    }

    @Test("transport errors are observed before proxy error mapping")
    func observesRawTransportError() async throws {
        let events = DiagnosticEvents()
        let context = try RunwayNetworkContext(
            configuration: .init(mode: .http, host: "localhost", port: 7890),
            sessionFactory: { configuration, delegate in
                configuration.protocolClasses = [DiagnosticURLProtocol.self]
                return URLSession(configuration: configuration, delegate: delegate, delegateQueue: nil)
            })
        await RunwayNetworkDiagnostics.$observer.withValue({ events.append($0) }) {
            do {
                _ = try await context.data(for: URLRequest(url: URL(string: "https://chatgpt.com/backend-api/wham/usage")!))
                Issue.record("Expected a proxy connection failure")
            } catch {
                #expect(error as? NetworkProxyError == .connectionFailed)
            }
        }
        let event = try #require(events.values.first)
        #expect(events.values.count == 1)
        #expect(event.operation == "quota")
        #expect(event.route == .http)
        #expect(event.errorDomain == NSURLErrorDomain)
        #expect(event.errorCode == URLError.timedOut.rawValue)
    }

    @Test("injected sessions report HTTP status without response content")
    func observesInjectedSession() async throws {
        let events = DiagnosticEvents()
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [DiagnosticURLProtocol.self]
        let session = URLSession(configuration: configuration)
        defer { session.invalidateAndCancel() }
        try await RunwayNetworkDiagnostics.$observer.withValue({ events.append($0) }) {
            _ = try await RunwayNetwork.data(
                for: URLRequest(url: URL(string: "https://chatgpt.com/backend-api/wham/rate-limit-reset-credits")!), session: session)
        }
        let event = try #require(events.values.first)
        #expect(events.values.count == 1)
        #expect(event.operation == "resetCredits")
        #expect(event.route == .injected)
        #expect(event.status == 503)
        #expect(event.errorDomain == nil)
        #expect(!event.line.contains("fixture-response"))
    }
}

private final class DiagnosticEvents: @unchecked Sendable {
    private let lock = NSLock()
    private var events: [RunwayNetworkDiagnostics.Event] = []
    var values: [RunwayNetworkDiagnostics.Event] { lock.withLock { events } }
    func append(_ event: RunwayNetworkDiagnostics.Event) { lock.withLock { events.append(event) } }
}

private final class DiagnosticURLProtocol: URLProtocol, @unchecked Sendable {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        if request.url?.path == "/backend-api/wham/usage" {
            client?.urlProtocol(self, didFailWithError: URLError(.timedOut))
        } else {
            let response = HTTPURLResponse(url: request.url!, statusCode: 503, httpVersion: nil, headerFields: nil)!
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: Data("fixture-response".utf8))
            client?.urlProtocolDidFinishLoading(self)
        }
    }
    override func stopLoading() {}
}
