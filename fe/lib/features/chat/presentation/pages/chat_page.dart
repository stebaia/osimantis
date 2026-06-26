import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/di/injection.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../graph/presentation/pages/graph_page.dart';
import '../../../person/presentation/widgets/person_card_view.dart';
import '../bloc/chat_bloc.dart';
import '../widgets/voice_blob.dart';

/// Home blob-first.
///
/// - All'avvio il blob è centrale e grande; **tieni premuto** per registrare
///   (rilascio = stop + invio).
/// - I controlli di testo (campo + mic + invio) restano NASCOSTI finché non apri
///   la tastiera (swipe su o tap sul blob): solo allora compare il composer e il
///   blob sale in alto rimpicciolendosi.
/// - Sotto compare la scheda della persona di cui si sta parlando.
class ChatPage extends StatelessWidget {
  const ChatPage({super.key});

  @override
  Widget build(BuildContext context) {
    return BlocProvider(
      create: (_) => sl<ChatBloc>(),
      child: const _ChatView(),
    );
  }
}

class _ChatView extends StatefulWidget {
  const _ChatView();

  @override
  State<_ChatView> createState() => _ChatViewState();
}

class _ChatViewState extends State<_ChatView> {
  final _controller = TextEditingController();
  final _inputFocus = FocusNode();

  /// Visibilità del composer, controllata da noi (swipe/tap la apre). È distinta
  /// da "tastiera di sistema aperta": il TextField deve essere visibile PRIMA di
  /// poter ricevere il focus e far comparire la tastiera.
  bool _inputVisible = false;

  @override
  void initState() {
    super.initState();
    // Quando il TextField perde il focus (es. l'utente chiude la tastiera con il
    // tasto back), nascondiamo di nuovo il composer.
    _inputFocus.addListener(() {
      if (!_inputFocus.hasFocus && _inputVisible) {
        setState(() => _inputVisible = false);
      }
    });
  }

  @override
  void dispose() {
    _controller.dispose();
    _inputFocus.dispose();
    super.dispose();
  }

  void _send(BuildContext context) {
    final text = _controller.text;
    if (text.trim().isEmpty) return;
    context.read<ChatBloc>().add(ChatMessageSent(text));
    _controller.clear();
  }

  /// Mostra il composer e poi richiede il focus (nello stesso frame il TextField
  /// è montato e visibile), così la tastiera si apre davvero.
  void _openKeyboard() {
    setState(() => _inputVisible = true);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _inputFocus.requestFocus();
    });
  }

  /// Chiude la tastiera togliendo il focus: il listener su _inputFocus rimette
  /// _inputVisible = false e nasconde il composer. unfocus() chiude anche la
  /// tastiera di sistema.
  void _closeKeyboard() {
    FocusManager.instance.primaryFocus?.unfocus();
    _inputFocus.unfocus();
  }

  @override
  Widget build(BuildContext context) {
    final keyboardOpen = _inputVisible;

    return Scaffold(
      
      body: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onVerticalDragEnd: (d) {
          if ((d.primaryVelocity ?? 0) < -100) _openKeyboard();
        },
        child: SafeArea(
        child: Stack(
          children: [
        BlocConsumer<ChatBloc, ChatState>(
          listenWhen: (p, c) =>
              p.errorMessage != c.errorMessage && c.errorMessage != null,
          listener: (context, state) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text(state.errorMessage!)),
            );
          },
          builder: (context, state) {
            void hold(bool on) => context
                .read<ChatBloc>()
                .add(ChatListeningToggled(listening: on));

            return Column(
              children: [
                // Header del blob: mini-barra a tastiera aperta, compatto se c'è
                // una scheda, grande e centrale altrimenti.
                if (keyboardOpen)
                  _BlobMiniBar(
                    state: state,
                    onHoldStart: () => hold(true),
                    onHoldEnd: () => hold(false),
                  )
                else if (state.activePerson != null)
                  _BlobHeader(
                    state: state,
                    onSwipeUp: _openKeyboard,
                    onHoldStart: () => hold(true),
                    onHoldEnd: () => hold(false),
                  ),
                Expanded(
                  child: keyboardOpen || state.activePerson != null
                      // Tap fuori dal campo → chiude la tastiera.
                      ? GestureDetector(
                          behavior: HitTestBehavior.opaque,
                          onTap: keyboardOpen ? _closeKeyboard : null,
                          child: _Body(state: state),
                        )
                      : _BlobHero(
                          state: state,
                          onSwipeUp: _openKeyboard,
                          onHoldStart: () => hold(true),
                          onHoldEnd: () => hold(false),
                        ),
                ),
                // I controlli di testo compaiono solo quando apriamo l'input
                // (swipe su / tap sul blob).
                if (keyboardOpen)
                  _Composer(
                    controller: _controller,
                    focusNode: _inputFocus,
                    state: state,
                    onSend: () => _send(context),
                    onMic: () => hold(!state.isListening),
                  ),
              ],
            );
          },
        ),
            // Accesso allo Spazio (grafo). Nascosto a tastiera aperta per non
            // disturbare il composer.
            if (!keyboardOpen)
              Positioned(
                top: 4,
                right: 8,
                child: IconButton(
                  tooltip: 'Spazio',
                  icon: const Icon(Icons.hub_outlined, color: AppColors.textSecondary),
                  onPressed: () => Navigator.of(context).push(
                    MaterialPageRoute<void>(builder: (_) => const GraphPage()),
                  ),
                ),
              ),
          ],
        ),
        ),
      ),
    );
  }
}

/// Gesture SUL BLOB: tap (apri tastiera) e press-and-hold (registra). Lo swipe-up
/// è gestito a livello di schermata (vedi il GestureDetector attorno al body).
class _BlobGesture extends StatelessWidget {
  const _BlobGesture({
    required this.child,
    required this.onTap,
    required this.onHoldStart,
    required this.onHoldEnd,
  });

  final Widget child;
  final VoidCallback onTap;
  final VoidCallback onHoldStart;
  final VoidCallback onHoldEnd;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      // Soglia più bassa del default (~500ms) per far partire prima la
      // registrazione tenendo premuto.
      onLongPressStart: (_) => onHoldStart(),
      onLongPressEnd: (_) => onHoldEnd(),
      child: child,
    );
  }
}

/// Stato iniziale: blob grande e centrale, con hint sotto.
class _BlobHero extends StatelessWidget {
  const _BlobHero({
    required this.state,
    required this.onSwipeUp,
    required this.onHoldStart,
    required this.onHoldEnd,
  });

  final ChatState state;
  final VoidCallback onSwipeUp;
  final VoidCallback onHoldStart;
  final VoidCallback onHoldEnd;

  @override
  Widget build(BuildContext context) {
    final busy = state.isBusy;
    final showTranscript = state.transcript.isNotEmpty;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          if (state.lastReply.isNotEmpty && !state.isListening) ...[
            Text(
              state.lastReply,
              textAlign: TextAlign.center,
              style: const TextStyle(
                fontSize: 20,
                height: 1.35,
                color: AppColors.textPrimary,
              ),
            ),
            const SizedBox(height: 28),
          ],
          Center(
            child: _BlobGesture(
              onTap: onSwipeUp,
              onHoldStart: onHoldStart,
              onHoldEnd: onHoldEnd,
              child: VoiceBlob(active: busy, size: 220),
            ),
          ),
          const SizedBox(height: 20),
          Text(
            showTranscript
                ? state.transcript
                : state.isListening
                    ? 'Ascolto...'
                    : 'Tieni premuto per parlare · swipe su per scrivere',
            textAlign: TextAlign.center,
            style: const TextStyle(color: AppColors.textSecondary),
          ),
          if (state.isListening) ...[
            const SizedBox(height: 20),
            _StopButton(onTap: onHoldEnd),
          ],
        ],
      ),
    );
  }
}

/// Pulsante per fermare la registrazione (oltre al rilascio dell'hold).
class _StopButton extends StatelessWidget {
  const _StopButton({required this.onTap});
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.primary,
      shape: const CircleBorder(),
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: const SizedBox(
          width: 56,
          height: 56,
          child: Icon(Icons.stop, color: Colors.white, size: 28),
        ),
      ),
    );
  }
}

/// Blob compatto in alto + stato (trascrizione live o hint).
/// Barra sottile in alto a tastiera aperta: blob mini (~40px) + stato. Lascia il
/// massimo spazio alla scheda e al campo di testo. Tieni premuto per registrare.
class _BlobMiniBar extends StatelessWidget {
  const _BlobMiniBar({
    required this.state,
    required this.onHoldStart,
    required this.onHoldEnd,
  });

  final ChatState state;
  final VoidCallback onHoldStart;
  final VoidCallback onHoldEnd;

  @override
  Widget build(BuildContext context) {
    final busy = state.isBusy;
    final label = state.transcript.isNotEmpty
        ? state.transcript
        : state.isListening
            ? 'Ascolto...'
            : 'Tieni premuto per parlare';
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 6, 16, 6),
      child: Row(
        children: [
          _BlobGesture(
            onTap: onHoldStart, // un tap sul mini-blob avvia comunque l'ascolto
            onHoldStart: onHoldStart,
            onHoldEnd: onHoldEnd,
            child: SizedBox(
              width: 44,
              height: 44,
              child: VoiceBlob(active: busy, size: 44),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(color: AppColors.textSecondary),
            ),
          ),
          if (state.isListening)
            IconButton(
              onPressed: onHoldEnd,
              icon: const Icon(Icons.stop_circle, color: AppColors.primary),
            ),
        ],
      ),
    );
  }
}

class _BlobHeader extends StatelessWidget {
  const _BlobHeader({
    required this.state,
    required this.onSwipeUp,
    required this.onHoldStart,
    required this.onHoldEnd,
  });

  final ChatState state;
  final VoidCallback onSwipeUp;
  final VoidCallback onHoldStart;
  final VoidCallback onHoldEnd;

  @override
  Widget build(BuildContext context) {
    final busy = state.isBusy;
    final showTranscript = state.transcript.isNotEmpty;
    return Padding(
      padding: const EdgeInsets.only(top: 8, bottom: 4),
      child: Column(
        children: [
          Center(
            child: _BlobGesture(
              onTap: onSwipeUp,
              onHoldStart: onHoldStart,
              onHoldEnd: onHoldEnd,
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 300),
                curve: Curves.easeOut,
                height: busy ? 140 : 96,
                alignment: Alignment.center,
                child: VoiceBlob(active: busy, size: busy ? 130 : 88),
              ),
            ),
          ),
          const SizedBox(height: 4),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 24),
            child: Text(
              showTranscript
                  ? state.transcript
                  : state.isListening
                      ? 'Ascolto...'
                      : 'Tieni premuto per parlare',
              textAlign: TextAlign.center,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(color: AppColors.textSecondary),
            ),
          ),
          if (state.isListening) ...[
            const SizedBox(height: 8),
            _StopButton(onTap: onHoldEnd),
          ],
        ],
      ),
    );
  }
}

/// Sotto al blob: scheda persona attiva, oppure stato vuoto.
class _Body extends StatelessWidget {
  const _Body({required this.state});
  final ChatState state;

  @override
  Widget build(BuildContext context) {
    if (state.personStatus == PersonStatus.loading && state.activePerson == null) {
      return const Center(child: CircularProgressIndicator());
    }
    final person = state.activePerson;
    if (person != null) {
      return PersonCardView(
        person: person,
        onShowInGraph: () => Navigator.of(context).push(
          MaterialPageRoute<void>(
            builder: (_) => GraphPage(highlightIds: {person.id}),
          ),
        ),
      );
    }
    return _Empty(state: state);
  }
}

class _Empty extends StatelessWidget {
  const _Empty({required this.state});
  final ChatState state;

  @override
  Widget build(BuildContext context) {
    final hasReply = state.lastReply.isNotEmpty;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Text(
            hasReply ? state.lastReply : 'Raccontami qualcosa\nsulle tue relazioni',
            textAlign: TextAlign.center,
            style: const TextStyle(
              fontSize: 22,
              height: 1.35,
              color: AppColors.textPrimary,
            ),
          ),
        ],
      ),
    );
  }
}

/// Barra in basso: campo testo + microfono + invio. Mostrata solo a tastiera aperta.
class _Composer extends StatelessWidget {
  const _Composer({
    required this.controller,
    required this.focusNode,
    required this.state,
    required this.onSend,
    required this.onMic,
  });

  final TextEditingController controller;
  final FocusNode focusNode;
  final ChatState state;
  final VoidCallback onSend;
  final VoidCallback onMic;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
      child: Row(
        children: [
          Expanded(
            child: TextField(
              controller: controller,
              focusNode: focusNode,
              minLines: 1,
              maxLines: 4,
              textInputAction: TextInputAction.send,
              onSubmitted: (_) => onSend(),
              decoration: InputDecoration(
                hintText: 'Scrivi un messaggio...',
                filled: true,
                fillColor: AppColors.surface,
                contentPadding:
                    const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(24),
                  borderSide: BorderSide.none,
                ),
              ),
            ),
          ),
          const SizedBox(width: 8),
          _RoundAction(
            icon: state.isListening ? Icons.stop : Icons.mic,
            highlighted: state.isListening,
            onTap: onMic,
          ),
          const SizedBox(width: 8),
          _RoundAction(
            icon: Icons.arrow_upward,
            highlighted: true,
            onTap: state.isSending ? null : onSend,
          ),
        ],
      ),
    );
  }
}

class _RoundAction extends StatelessWidget {
  const _RoundAction({
    required this.icon,
    required this.onTap,
    this.highlighted = false,
  });
  final IconData icon;
  final VoidCallback? onTap;
  final bool highlighted;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: highlighted ? AppColors.primary : AppColors.surface,
      shape: const CircleBorder(),
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: SizedBox(
          width: 48,
          height: 48,
          child: Icon(icon, color: highlighted ? Colors.white : AppColors.textPrimary),
        ),
      ),
    );
  }
}
