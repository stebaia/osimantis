import 'package:dio/dio.dart';

import '../error/failure.dart';

/// Mappa un DioException nella Failure di dominio corrispondente. Centralizzata
/// qui così ogni datasource la riusa senza duplicare la logica.
Failure mapDioError(DioException e) {
  switch (e.type) {
    case DioExceptionType.connectionTimeout:
    case DioExceptionType.sendTimeout:
    case DioExceptionType.receiveTimeout:
    case DioExceptionType.connectionError:
      return const NetworkFailure();
    case DioExceptionType.badResponse:
      final status = e.response?.statusCode ?? 0;
      if (status == 401) return const AuthFailure();
      return ServerFailure(_serverMessage(e) ?? 'Errore del server ($status)');
    default:
      return const ServerFailure();
  }
}

/// Estrae il messaggio d'errore dal corpo JSON del backend ({"error": "..."}).
String? _serverMessage(DioException e) {
  final data = e.response?.data;
  if (data is Map && data['error'] is String) return data['error'] as String;
  return null;
}
