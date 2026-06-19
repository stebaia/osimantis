/// Configurazione iniettata a build-time tramite --dart-define.
///
/// Esempio:
///   flutter run \
///     --dart-define=BASE_URL=https://osimantis.tuodominio.it \
///     --dart-define=API_TOKEN=il-tuo-token
///
/// I default puntano al backend in locale (emulatore Android: 10.0.2.2).
class AppConfig {
  const AppConfig._();

  /// URL base del backend Go (senza slash finale). Default: produzione su Coolify.
  /// Per il backend locale: --dart-define=BASE_URL=http://10.0.2.2:8080 (emulatore
  /// Android) o `http://IP-del-Mac:8080` (device fisico).
  static const String baseUrl = String.fromEnvironment(
    'BASE_URL',
    defaultValue: 'https://osimantis.yoocoding.com',
  );

  /// Bearer token per l'autenticazione (corrisponde ad API_TOKEN del backend).
  static const String apiToken = String.fromEnvironment(
    'API_TOKEN',
    defaultValue: '',
  );
}
