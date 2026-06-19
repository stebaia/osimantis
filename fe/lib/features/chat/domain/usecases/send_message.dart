import '../../../../core/error/result.dart';
import '../entities/chat_message.dart';
import '../repositories/chat_repository.dart';

/// Usecase: invia un messaggio all'agente, con lo storico della conversazione.
/// Sottile, ma tiene il bloc disaccoppiato dal repository.
class SendMessage {
  const SendMessage(this._repository);
  final ChatRepository _repository;

  Future<Result<String>> call(
    String text, {
    List<ChatMessage> history = const [],
  }) => _repository.sendMessage(text, history: history);
}
