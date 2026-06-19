part of 'chat_bloc.dart';

/// Stato della schermata chat. status guida l'UI (idle/ascolto/invio); messages
/// è lo storico; transcript è il testo dettato in tempo reale.
class ChatState extends Equatable {
  const ChatState({
    this.messages = const [],
    this.status = ChatStatus.idle,
    this.transcript = '',
    this.errorMessage,
  });

  final List<ChatMessage> messages;
  final ChatStatus status;
  final String transcript;
  final String? errorMessage;

  bool get isListening => status == ChatStatus.listening;
  bool get isSending => status == ChatStatus.sending;

  ChatState copyWith({
    List<ChatMessage>? messages,
    ChatStatus? status,
    String? transcript,
    String? errorMessage,
    bool clearError = false,
  }) {
    return ChatState(
      messages: messages ?? this.messages,
      status: status ?? this.status,
      transcript: transcript ?? this.transcript,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
    );
  }

  @override
  List<Object?> get props => [messages, status, transcript, errorMessage];
}

enum ChatStatus { idle, listening, sending }
