import CryptoKit
import Foundation

// Keep the private key on stdin; never pass it through argv or print it.
let input = FileHandle.standardInput.readDataToEndOfFile()
let encoded = String(data: input, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
guard let seed = Data(base64Encoded: encoded),
      let key = try? Curve25519.Signing.PrivateKey(rawRepresentation: seed),
      key.publicKey.rawRepresentation.base64EncodedString() == ProcessInfo.processInfo.environment["SPARKLE_PUBLIC_KEY"]
else {
    FileHandle.standardError.write(Data("Sparkle signing keys are missing, invalid, or do not match.\n".utf8))
    exit(1)
}
print("Sparkle signing key pair verified.")
