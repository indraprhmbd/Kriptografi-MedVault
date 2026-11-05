"""USB Dongle Key Generator for MedVault Crypto.

Generates a cryptographically secure 32-byte hex token, writes it
to auth.key on a specified directory or USB drive mount point with
hidden and system attributes, and prints the SHA-256 hash required
for user registration.
"""

import argparse
from pathlib import Path
import sys

# Add project root to sys.path
PROJECT_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(PROJECT_ROOT))

from backend.config import settings
from backend.core.dongle_auth import DongleAuthenticator, is_hidden_system_file


def generate_dongle_key(target_dir: Path, overwrite: bool = True) -> tuple[str, str]:
    """Generate auth.key at target directory with hidden+system attributes.

    Returns (raw_token, sha256_hash).
    """
    authenticator = DongleAuthenticator()
    raw_token, token_hash, _ = authenticator.provision_dongle_key(
        target_dir=target_dir,
        overwrite=overwrite,
    )
    return raw_token, token_hash


def main() -> None:
    parser = argparse.ArgumentParser(
        description="MedVault Crypto - Pembuat Kunci USB Dongle Fisik (auth.key)"
    )
    parser.add_argument(
        "--path",
        type=str,
        default=None,
        help="Direktori target atau mount point USB drive (contoh: E:\\ atau storage/mock_usb/)",
    )
    parser.add_argument(
        "--no-overwrite",
        action="store_true",
        help="Jangan menimpa file auth.key jika sudah ada di direktori target",
    )
    args = parser.parse_args()

    if args.path:
        target_path = Path(args.path)
    else:
        # Default fallback to mock usb directory inside storage
        target_path = settings.storage_dir_path / "mock_usb"

    raw_token, token_hash = generate_dongle_key(target_path, overwrite=not args.no_overwrite)
    key_file = target_path / settings.MEDVAULT_DONGLE_FILENAME
    is_protected = is_hidden_system_file(key_file)

    print("[MedVault] File kunci USB Dongle berhasil dibuat.")
    print(f"Lokasi File         : {key_file.resolve()}")
    print(f"Atribut Tersembunyi : {'Ya (Hidden + System File)' if is_protected else 'Standar'}")
    print(f"Token Asli          : {raw_token}")
    print(f"SHA-256 Hash        : {token_hash}")
    print("\nSimpan SHA-256 Hash di atas untuk didaftarkan pada kolom 'dongle_token_hash_hex' akun dokter.")


if __name__ == "__main__":
    main()
