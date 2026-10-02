import Foundation

/// One async fixture request owns one callback target. Only its response task invokes these
/// callbacks; the fixture never mutates URLProtocol state across tasks. This explicit boundary
/// also works with newer Foundation SDKs where URLProtocol's Sendable conformance is unavailable.
struct MockURLProtocolResponse: @unchecked Sendable {
    private let instance: URLProtocol
    private let client: (any URLProtocolClient)?

    init(_ instance: URLProtocol) {
        self.instance = instance
        self.client = instance.client
    }

    func receive(_ response: HTTPURLResponse, data: Data) {
        client?.urlProtocol(instance, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(instance, didLoad: data)
        client?.urlProtocolDidFinishLoading(instance)
    }

    func fail(_ error: Error) {
        client?.urlProtocol(instance, didFailWithError: error)
    }
}
