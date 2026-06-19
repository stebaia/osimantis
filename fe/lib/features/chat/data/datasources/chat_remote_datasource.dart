import 'package:dio/dio.dart';

/// Datasource remoto della chat: parla con POST /chat del backend Go.
class ChatRemoteDataSource {
  ChatRemoteDataSource(this._dio);
  final Dio _dio;

  /// POST /chat con body {"text": ..., "history": [...]} → ritorna "reply".
  /// history è lo storico della conversazione (memoria a breve termine), così
  /// l'agente mantiene il filo del discorso. Propaga DioException: la mappatura
  /// a Failure la fa il repository.
  Future<String> sendMessage(
    String text, {
    List<Map<String, String>> history = const [],
  }) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/chat',
      data: {'text': text, 'history': history},
    );
    final reply = res.data?['reply'];
    if (reply is String) return reply;
    throw DioException(
      requestOptions: res.requestOptions,
      response: res,
      type: DioExceptionType.badResponse,
      error: 'Risposta priva del campo "reply"',
    );
  }
}
