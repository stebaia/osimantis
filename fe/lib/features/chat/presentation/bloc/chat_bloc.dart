// I bloc espongono parametri pubblici nel costruttore ma tengono i campi
// privati: l'initializing-formal qui ridurrebbe la leggibilità della DI.
// ignore_for_file: prefer_initializing_formals
import 'package:equatable/equatable.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/error/result.dart';
import '../../../../core/speech/speech_service.dart';
import '../../../person/domain/entities/person_card.dart';
import '../../../person/domain/usecases/get_person.dart';
import '../../domain/entities/chat_message.dart';
import '../../domain/entities/chat_reply.dart';
import '../../domain/usecases/send_message.dart';

part 'chat_event.dart';
part 'chat_state.dart';

/// Orchestrazione della home blob-first: dettatura vocale (SpeechService), invio
/// all'agente (SendMessage) e caricamento della scheda della persona toccata
/// (GetPerson). Lo storico dei messaggi è tenuto solo come memoria conversazionale
/// per il backend, NON viene reso come chat.
class ChatBloc extends Bloc<ChatEvent, ChatState> {
  ChatBloc({
    required SendMessage sendMessage,
    required GetPerson getPerson,
    required SpeechService speech,
  }) : _sendMessage = sendMessage,
       _getPerson = getPerson,
       _speech = speech,
       super(const ChatState()) {
    on<ChatListeningToggled>(_onListeningToggled);
    on<ChatTranscriptUpdated>(_onTranscriptUpdated);
    on<ChatMessageSent>(_onMessageSent);
  }

  final SendMessage _sendMessage;
  final GetPerson _getPerson;
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

    // Storico (memoria del discorso) catturato PRIMA di aggiungere il messaggio
    // corrente. Il backend lo usa per mantenere il filo (es. capire "lui/lei").
    final history = List<ChatMessage>.from(state.messages);

    emit(state.copyWith(
      messages: [...history, ChatMessage(role: ChatRole.user, text: text)],
      status: ChatStatus.sending,
      transcript: '',
      clearError: true,
    ));

    final result = await _sendMessage(text, history: history);

    switch (result) {
      case Error(:final failure):
        emit(state.copyWith(status: ChatStatus.idle, errorMessage: failure.message));
      case Success(:final data):
        // Aggiungiamo la risposta allo storico (per i turni futuri) e mostriamo il
        // testo breve; la scheda della persona toccata la carichiamo a parte.
        emit(state.copyWith(
          messages: [
            ...state.messages,
            ChatMessage(role: ChatRole.assistant, text: data.text),
          ],
          status: ChatStatus.idle,
          lastReply: data.text,
        ));
        await _loadTouchedPerson(data, emit);
    }
  }

  /// Se il turno ha toccato almeno una persona, carica la scheda dell'ultima e la
  /// rende attiva sotto al blob. Se il caricamento fallisce, la scheda precedente
  /// resta invariata (l'errore non è bloccante: la reply è già mostrata).
  Future<void> _loadTouchedPerson(ChatReply reply, Emitter<ChatState> emit) async {
    final touched = reply.lastTouched;
    if (touched == null) return;

    emit(state.copyWith(personStatus: PersonStatus.loading));
    final result = await _getPerson(touched.id);
    switch (result) {
      case Success(:final data):
        emit(state.copyWith(activePerson: data, personStatus: PersonStatus.loaded));
      case Error():
        emit(state.copyWith(
          personStatus: state.activePerson != null
              ? PersonStatus.loaded
              : PersonStatus.none,
        ));
    }
  }
}
