part of 'chat_bloc.dart';

sealed class ChatEvent extends Equatable {
  const ChatEvent();

  @override
  List<Object?> get props => [];
}

/// L'utente invia un testo all'agente.
class ChatMessageSent extends ChatEvent {
  const ChatMessageSent(this.text);
  final String text;

  @override
  List<Object?> get props => [text];
}

/// Aggiorna il testo "in corso di dettatura" mentre il microfono ascolta.
class ChatTranscriptUpdated extends ChatEvent {
  const ChatTranscriptUpdated(this.transcript);
  final String transcript;

  @override
  List<Object?> get props => [transcript];
}

/// Avvia/ferma l'ascolto vocale (toggle del microfono).
class ChatListeningToggled extends ChatEvent {
  const ChatListeningToggled({required this.listening});
  final bool listening;

  @override
  List<Object?> get props => [listening];
}
