import 'package:dio/dio.dart';

import '../config/app_config.dart';

/// Crea il Dio configurato verso il backend Osimantis: base URL, timeout e
/// header `Authorization: Bearer <token>` su ogni richiesta.
///
/// Il token arriva da AppConfig (dart-define). In futuro, se passiamo a una
/// schermata impostazioni, basta sostituire la sorgente del token qui.
Dio createDio() {
  final dio = Dio(
    BaseOptions(
      baseUrl: AppConfig.baseUrl,
      connectTimeout: const Duration(seconds: 10),
      // L'agente LLM può essere lento: timeout di ricezione generoso.
      receiveTimeout: const Duration(seconds: 70),
      contentType: Headers.jsonContentType,
    ),
  );

  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) {
        if (AppConfig.apiToken.isNotEmpty) {
          options.headers['Authorization'] = 'Bearer ${AppConfig.apiToken}';
        }
        handler.next(options);
      },
    ),
  );

  return dio;
}
