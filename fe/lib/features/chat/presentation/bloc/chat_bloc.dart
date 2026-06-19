// I bloc espongono parametri pubblici nel costruttore ma tengono i campi
// privati: l'initializing-formal qui ridurrebbe la leggibilità della DI.
// ignore_for_file: prefer_initializing_formals
import 'package:equatable/equatable.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/error/result.dart';
import '../../../../core/speech/speech_service.dart';
import '../../domain/entities/chat_message.dart';
import '../../domain/usecases/send_message.dart';

part 'chat_event.dart';
part 'chat_state.dart';

/// Orchestrazione della schermata chat: dettatura vocale (SpeechService) e invio
/// all'agente (SendMessage usecase).
class ChatBloc extends Bloc<ChatEvent, ChatState> {
  ChatBloc({
    required SendMessage sendMessage,
    required SpeechService speech,
  }) : _sendMessage = sendMessage,
       _speech = speech,
       super(const ChatState()) {
    on<ChatListeningToggled>(_onListeningToggled);
    on<ChatTranscriptUpdated>(_onTranscriptUpdated);
    on<ChatMessageSent>(_onMessageSent);
  }

  final SendMessage _sendMessage;
  final SpeechService _speech;

  Future<void> _onListeningToggled(
    ChatListeningToggled event,
    Emitter<ChatState> emit,
  ) async {
    if (event.listening) {
      final ok = await _speech.init();
      if (!ok) {
        emit(state.copyWith(errorMessage: 'Microfono non disponibile'));
        return;
      }
      emit(state.copyWith(status: ChatStatus.listening, transcript: '', clearError: true));
      await _speech.start((t) => add(ChatTranscriptUpdated(t)));
    } else {
      await _speech.stop();
      final text = state.transcript.trim();
      emit(state.copyWith(status: ChatStatus.idle));
      // Allo stop, se è stato dettato qualcosa, lo inviamo.
      if (text.isNotEmpty) add(ChatMessageSent(text));
    }
  }

  void _onTranscriptUpdated(
    ChatTranscriptUpdated event,
    Emitter<ChatState> emit,
  ) {
    emit(state.copyWith(transcript: event.transcript));
  }

  Future<void> _onMessageSent(
    ChatMessageSent event,
    Emitter<ChatState> emit,
  ) async {
    final text = event.text.trim();
    if (text.isEmpty || state.isSending) return;

    // Lo storico = i messaggi già scambiati (memoria del discorso). Lo
    // catturiamo PRIMA di aggiungere il nuovo messaggio e la bolla pending.
    final history = state.messages.where((m) => !m.pending).toList();

    final messages = [
      ...state.messages,
      ChatMessage(role: ChatRole.user, text: text),
      const ChatMessage(role: ChatRole.assistant, text: '', pending: true),
    ];
    emit(state.copyWith(
      messages: messages,
      status: ChatStatus.sending,
      transcript: '',
      clearError: true,
    ));

    final result = await _sendMessage(text, history: history);

    final updated = [...state.messages];
    // L'ultimo messaggio è la bolla "pending" dell'assistente: la rimpiazziamo.
    final replyText = switch (result) {
      Success(:final data) => data,
      Error(:final failure) => failure.message,
    };
    updated[updated.length - 1] = ChatMessage(
      role: ChatRole.assistant,
      text: replyText,
    );

    emit(state.copyWith(
      messages: updated,
      status: ChatStatus.idle,
      errorMessage: result is Error<String> ? result.failure.message : null,
    ));
  }
}
