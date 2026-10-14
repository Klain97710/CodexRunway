import Foundation
import WidgetKit

// C ABI loaded into the Go host; WidgetCenter therefore sees the host bundle,
// rather than a helper executable with a different application identity.
@_cdecl("quosparr_widget_bridge_version")
public func runwayWidgetBridgeVersion() -> Int32 { 1 }

@_cdecl("quosparr_widget_query")
public func runwayWidgetQuery(_ request: UInt64, _ callbackPointer: UnsafeRawPointer) {
    let callback = unsafeBitCast(callbackPointer,
        to: (@convention(c) (UInt64, UnsafePointer<CChar>, Int) -> Void).self)
    DispatchQueue.main.async {
        guard #available(macOS 14.0, *) else {
            deliver(request, callback, ["error": "WidgetKit requires macOS 14 or later"])
            return
        }
        WidgetCenter.shared.getCurrentConfigurations { result in
            switch result {
            case .success(let configurations):
                let items = configurations.map { item -> [String: String] in
                    let family: String
                    switch item.family {
                    case .systemSmall: family = "small"
                    case .systemLarge: family = "large"
                    default: family = "medium"
                    }
                    return ["kind": item.kind, "family": family]
                }
                deliver(request, callback, ["configurations": items])
            case .failure(let error):
                deliver(request, callback, ["error": error.localizedDescription])
            }
        }
    }
}

private func deliver(_ request: UInt64,
    _ callback: @convention(c) (UInt64, UnsafePointer<CChar>, Int) -> Void,
    _ payload: [String: Any]) {
    do {
        let data = try JSONSerialization.data(withJSONObject: payload, options: [.sortedKeys])
        let string = String(decoding: data, as: UTF8.self)
        string.withCString { callback(request, $0, string.utf8.count) }
    } catch {
        // Encoding errors remain visible to the host; never report an empty
        // configuration list as a successful WidgetKit query.
        let message = "{\"error\":\"WidgetKit bridge could not encode configurations\"}"
        message.withCString { callback(request, $0, message.utf8.count) }
    }
}

@_cdecl("quosparr_widget_reload")
public func runwayWidgetReload(_ kindsJSON: UnsafePointer<CChar>) -> Int32 {
    guard #available(macOS 14.0, *) else { return 1 }
    do {
        let data = Data(String(cString: kindsJSON).utf8)
        let kinds = try JSONDecoder().decode([String].self, from: data)
        DispatchQueue.main.async {
            for kind in kinds { WidgetCenter.shared.reloadTimelines(ofKind: kind) }
        }
        return 0
    } catch {
        return 2
    }
}
