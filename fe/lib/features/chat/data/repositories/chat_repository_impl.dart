import 'package:dio/dio.dart';

import '../../../../core/error/failure.dart';
import '../../../../core/error/result.dart';
import '../../../../core/network/api_exception.dart';
import '../../domain/entities/chat_message.dart';
import '../../domain/repositories/chat_repository.dart';
import '../datasources/chat_remote_datasource.dart';

/// Implementazione del ChatRepository: delega al datasource e converte le
/// eccezioni tecniche in Result/Failure di dominio.
class ChatRepositoryImpl implements ChatRepository {
  ChatRepositoryImpl(this._remote);
  final ChatRemoteDataSource _remote;

  @override
  Future<Result<String>> sendMessage(
    String text, {
    List<ChatMessage> history = const [],
  }) async {
    try {
      final reply = await _remote.sendMessage(
        text,
        history: history.map(_toWire).toList(),
      );
      return Success(reply);
    } on DioException catch (e) {
      return Error(mapDioError(e));
    } catch (_) {
      return const Error(ServerFailure());
    }
  }

  /// Converte un ChatMessage nel formato {role, text} atteso dal backend.
  Map<String, String> _toWire(ChatMessage m) => {
    'role': m.role == ChatRole.user ? 'user' : 'assistant',
    'text': m.text,
  };
}
