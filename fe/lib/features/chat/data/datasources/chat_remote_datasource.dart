import 'package:dio/dio.dart';

/// Datasource remoto della chat: parla con POST /chat del backend Go.
class ChatRemoteDataSource {
  ChatRemoteDataSource(this._dio);
  final Dio _dio;

  /// POST /chat con body {"text": ..., "history": [...]} → ritorna il JSON grezzo
  /// {"reply": ..., "touched": [{id, name}]}. history è lo storico della
  /// conversazione (memoria a breve termine), così l'agente mantiene il filo del
  /// discorso. Propaga DioException: la mappatura a Failure e al dominio la fa il
  /// repository.
  Future<Map<String, dynamic>> sendMessage(
    String text, {
    List<Map<String, String>> history = const [],
  }) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/chat',
      data: {'text': text, 'history': history},
    );
    final data = res.data;
    if (data != null && data['reply'] is String) return data;
    throw DioException(
      requestOptions: res.requestOptions,
      response: res,
      type: DioExceptionType.badResponse,
      error: 'Risposta priva del campo "reply"',
    );
  }
}
