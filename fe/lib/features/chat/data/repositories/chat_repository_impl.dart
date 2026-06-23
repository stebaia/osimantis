import 'package:dio/dio.dart';

import '../../../../core/error/failure.dart';
import '../../../../core/error/result.dart';
import '../../../../core/network/api_exception.dart';
import '../../domain/entities/chat_message.dart';
import '../../domain/entities/chat_reply.dart';
import '../../domain/repositories/chat_repository.dart';
import '../datasources/chat_remote_datasource.dart';

/// Implementazione del ChatRepository: delega al datasource e converte le
/// eccezioni tecniche in Result/Failure di dominio.
class ChatRepositoryImpl implements ChatRepository {
  ChatRepositoryImpl(this._remote);
  final ChatRemoteDataSource _remote;

  @override
  Future<Result<ChatReply>> sendMessage(
    String text, {
    List<ChatMessage> history = const [],
  }) async {
    try {
      final json = await _remote.sendMessage(
        text,
        history: history.map(_toWire).toList(),
      );
      return Success(_replyFromJson(json));
    } on DioException catch (e) {
      return Error(mapDioError(e));
    } catch (_) {
      return const Error(ServerFailure());
    }
  }

  ChatReply _replyFromJson(Map<String, dynamic> json) {
    final touched = (json['touched'] as List?)
            ?.map((e) {
              final m = (e as Map).cast<String, dynamic>();
              return TouchedPerson(
                id: (m['id'] as num).toInt(),
                name: (m['name'] as String?) ?? '',
              );
            })
            .toList() ??
        const <TouchedPerson>[];
    return ChatReply(text: json['reply'] as String, touched: touched);
  }

  /// Converte un ChatMessage nel formato {role, text} atteso dal backend.
  Map<String, String> _toWire(ChatMessage m) => {
    'role': m.role == ChatRole.user ? 'user' : 'assistant',
    'text': m.text,
  };
}
