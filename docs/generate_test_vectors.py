# Generate ECDSA P-256 test vectors for ESP32 micro-ecc verification
# Outputs C-style byte arrays ready to paste into crypto.cpp
#
# Requirements: pip install ecdsa

import hashlib
from ecdsa import SigningKey, NIST256p, BadSignatureError

def bytes_to_c_array(data: bytes, name: str, per_line: int = 12) -> str:
    """Format bytes as a C uint8_t array."""
    lines = []
    lines.append(f"static const uint8_t {name}[] = {{")
    for i in range(0, len(data), per_line):
        chunk = data[i:i + per_line]
        hex_vals = ", ".join(f"0x{b:02X}" for b in chunk)
        comma = "," if i + per_line < len(data) else ""
        lines.append(f"    {hex_vals}{comma}")
    lines.append("};")
    return "\n".join(lines)


def main():
    # 1. Generate a P-256 keypair
    sk = SigningKey.generate(curve=NIST256p)
    vk = sk.get_verifying_key()

    # Public key: 64 bytes (32 bytes X + 32 bytes Y), no 0x04 prefix
    pubkey_bytes = vk.to_string()  # 64 bytes
    assert len(pubkey_bytes) == 64

    # 2. Build a test payload matching the contract:
    #    UID(7) || version(1) || emisor(1) || atributo(1) || largo(1) || vence(4) || handle(4) || valor(N)
    uid = bytes([0x04, 0xA3, 0xB5, 0xC7, 0xD9, 0xE1, 0xF0])
    version = bytes([0x01])
    emisor = bytes([0x01])         # Issuer ID 1
    atributo = bytes([0xA1])       # A1 = age verification
    valor = b"MAYOR_DE_EDAD"       # "of legal age"
    largo = bytes([len(valor)])
    vence = (1735689600).to_bytes(4, "big")  # 2025-01-01 00:00:00 UTC
    handle = (0x0000ABCD).to_bytes(4, "big")

    payload = uid + version + emisor + atributo + largo + vence + handle + valor
    print(f"Payload length: {len(payload)} bytes")
    print(f"Payload hex: {payload.hex()}")

    # 3. SHA-256 of the payload
    digest = hashlib.sha256(payload).digest()
    print(f"SHA-256: {digest.hex()}")

    # 4. Sign the hash with ECDSA P-256 (deterministic RFC 6979)
    signature = sk.sign_digest(digest, sigencode=lambda r, s, order: r.to_bytes(32, "big") + s.to_bytes(32, "big"))
    assert len(signature) == 64

    # 5. Verify it works
    try:
        vk.verify_digest(signature, digest, sigdecode=lambda sig, order: (int.from_bytes(sig[:32], "big"), int.from_bytes(sig[32:], "big")))
        print("Signature verification: PASS")
    except BadSignatureError:
        print("Signature verification: FAIL (BUG!)")
        return

    # 6. Output C arrays
    print("\n// === Copy everything below into crypto.cpp test vectors ===\n")
    print(bytes_to_c_array(pubkey_bytes, "TEST_PUBKEY"))
    print()
    print(bytes_to_c_array(payload, "TEST_PAYLOAD"))
    print(f"static const size_t TEST_PAYLOAD_LEN = {len(payload)};")
    print()
    print(bytes_to_c_array(digest, "TEST_HASH"))
    print()
    print(bytes_to_c_array(signature, "TEST_SIGNATURE"))
    print()

    # 7. Also output a WRONG signature for negative test
    wrong_sig = bytearray(signature)
    wrong_sig[0] ^= 0xFF  # flip first byte
    print(bytes_to_c_array(bytes(wrong_sig), "TEST_SIGNATURE_BAD"))
    print()

    print(f"// Private key (for reference only, do NOT embed in firmware):")
    print(f"// {sk.to_string().hex()}")


if __name__ == "__main__":
    main()
